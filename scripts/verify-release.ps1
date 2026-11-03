param(
    [string]$PostgresContainer = 'compose-postgres-1',
    [string]$PostgresUser = 'timetracker',
    [string]$DatabaseHost = 'localhost',
    [int]$DatabasePort = 5432,
    [string]$DatabasePassword = 'timetracker',
    [switch]$SkipRace,
    [switch]$LinuxRace
)

# Creates only fresh synthetic databases; never migrates or restores the app DB.
$ErrorActionPreference = 'Stop'
if ($SkipRace -and $LinuxRace) { throw 'Choose either SkipRace or LinuxRace, not both.' }
$repository = Split-Path -Parent $PSScriptRoot
$suffix = [guid]::NewGuid().ToString('N')
$testDatabase = "tt_release_$suffix"
$restoreDatabase = "tt_restore_$suffix"
$runtimeRole = "tt_runtime_$suffix"
$createdRole = $false
$dumpPath = "/tmp/tt_release_$suffix.dump"
$createdDatabases = [System.Collections.Generic.List[string]]::new()
$previousDatabaseURL = $env:DATABASE_URL

function Assert-Exit([string]$Step) {
    if ($LASTEXITCODE -ne 0) { throw "$Step failed (exit $LASTEXITCODE)" }
}
function Invoke-SQL([string]$Database, [string]$SQL) {
    $SQL | docker exec -i $PostgresContainer psql -U $PostgresUser -d $Database -v ON_ERROR_STOP=1 -q
    Assert-Exit "SQL on $Database"
}

try {
    foreach ($database in @($testDatabase, $restoreDatabase)) {
        docker exec $PostgresContainer createdb -U $PostgresUser $database
        Assert-Exit 'Create disposable database'
        $createdDatabases.Add($database)
    }
    $migrations = Get-ChildItem -LiteralPath (Join-Path $repository 'db/migrations') -Filter '*.up.sql' | Sort-Object Name
    foreach ($migration in $migrations) {
        if ($migration.Name -ge '000009') { continue }
        Invoke-SQL $testDatabase (Get-Content -LiteralPath $migration.FullName -Raw)
    }
    # A real pre-amendment ticket link must survive the forward migration.
    Invoke-SQL $testDatabase @'
INSERT INTO users(id,email,password_hash,name,timezone,email_verified_at) VALUES('10000000-0000-4000-8000-000000000001','restore@example.test','synthetic','Restore tester','UTC',CURRENT_TIMESTAMP);
INSERT INTO organizations(id,name,timezone) VALUES('10000000-0000-4000-8000-000000000002','Release rehearsal','UTC');
INSERT INTO memberships(id,organization_id,user_id,role) VALUES('10000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','ADMIN');
INSERT INTO projects(id,organization_id,name) VALUES('10000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000002','Rehearsal');
INSERT INTO tickets(id,organization_id,created_by_user_id,reference,title) VALUES('10000000-0000-4000-8000-000000000005','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','MIG-1','Preserved link');
INSERT INTO time_entries(id,organization_id,user_id,project_id,ticket_id,description,source_type,status,started_at,ended_at,duration_seconds) VALUES('10000000-0000-4000-8000-000000000006','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000005','Preserved entry','MANUAL','STOPPED','2026-01-01 09:00Z','2026-01-01 10:00Z',3600);
'@
    foreach ($migration in $migrations) {
        if ($migration.Name -lt '000009') { continue }
        Invoke-SQL $testDatabase (Get-Content -LiteralPath $migration.FullName -Raw)
    }
    $assertions = @'
DO $$ BEGIN
IF (SELECT count(*) FROM users) <> 1 OR (SELECT count(*) FROM organizations) <> 1 OR (SELECT count(*) FROM time_entries) <> 1 OR (SELECT count(*) FROM tickets) <> 1 THEN RAISE EXCEPTION 'Unexpected synthetic fixture count'; END IF;
IF NOT EXISTS(SELECT 1 FROM time_entry_tickets WHERE time_entry_id='10000000-0000-4000-8000-000000000006' AND ticket_id='10000000-0000-4000-8000-000000000005') THEN RAISE EXCEPTION 'Existing link lost'; END IF;
IF (SELECT duration_seconds FROM time_entries WHERE id='10000000-0000-4000-8000-000000000006') <> 3600 THEN RAISE EXCEPTION 'Duration changed'; END IF;
IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='time_entries' AND column_name='ticket_id') THEN RAISE EXCEPTION 'Legacy column still exists'; END IF;
BEGIN
DELETE FROM tickets WHERE id='10000000-0000-4000-8000-000000000005';
RAISE EXCEPTION 'Linked ticket deleted';
EXCEPTION WHEN foreign_key_violation THEN NULL;
END;
END $$;
'@
    Invoke-SQL $testDatabase $assertions
    Invoke-SQL $testDatabase "CREATE ROLE $runtimeRole NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;"
    $createdRole = $true
    Get-Content -LiteralPath (Join-Path $PSScriptRoot 'runtime-grants.sql') -Raw | docker exec -i $PostgresContainer psql -U $PostgresUser -d $testDatabase -v ON_ERROR_STOP=1 -v "application_role=$runtimeRole" -q
    Assert-Exit 'Apply least-privilege runtime grants'
    $grantAssertions = @'
BEGIN;
SET LOCAL ROLE ROLE_PLACEHOLDER;
UPDATE time_entries SET description=description WHERE id='10000000-0000-4000-8000-000000000006';
DO $$ BEGIN
IF NOT has_table_privilege(current_user,'audit_logs','SELECT') OR NOT has_table_privilege(current_user,'audit_logs','INSERT') THEN RAISE EXCEPTION 'Audit read/append permissions missing'; END IF;
IF has_schema_privilege(current_user,'public','CREATE') THEN RAISE EXCEPTION 'Runtime can create schema objects'; END IF;
BEGIN UPDATE audit_logs SET action=action; RAISE EXCEPTION 'Runtime can update audit logs'; EXCEPTION WHEN insufficient_privilege THEN NULL; END;
BEGIN DELETE FROM audit_logs; RAISE EXCEPTION 'Runtime can delete audit logs'; EXCEPTION WHEN insufficient_privilege THEN NULL; END;
BEGIN TRUNCATE audit_logs; RAISE EXCEPTION 'Runtime can truncate audit logs'; EXCEPTION WHEN insufficient_privilege THEN NULL; END;
END $$;
ROLLBACK;
'@
    Invoke-SQL $testDatabase ($grantAssertions.Replace('ROLE_PLACEHOLDER', $runtimeRole))
    $builder = [System.UriBuilder]::new('postgres', $DatabaseHost, $DatabasePort, $testDatabase)
    $builder.UserName = $PostgresUser
    $builder.Password = $DatabasePassword
    $builder.Query = 'sslmode=disable'
    $env:DATABASE_URL = $builder.Uri.AbsoluteUri
    Push-Location (Join-Path $repository 'apps/api')
    try {
        go vet ./...
        Assert-Exit 'Go vet'
        if ($LinuxRace) {
            # Shared container network makes localhost point to this disposable PostgreSQL.
            $linuxURL = [System.UriBuilder]::new('postgres', 'localhost', 5432, $testDatabase)
            $linuxURL.UserName = $PostgresUser
            $linuxURL.Password = $DatabasePassword
            $linuxURL.Query = 'sslmode=disable'
            docker run --rm --network "container:$PostgresContainer" --mount "type=bind,source=$repository,target=/workspace,readonly" -w /workspace/apps/api -e "DATABASE_URL=$($linuxURL.Uri.AbsoluteUri)" -e GOCACHE=/tmp/go-cache golang:1.25-bookworm go test -count=1 -race ./...
        } elseif ($SkipRace) {
            Write-Warning 'Race detector explicitly skipped; Linux CI must run it before release.'
            go test -count=1 ./...
        } else {
            go test -count=1 -race ./...
        }
        Assert-Exit 'API integration and race tests'
    } finally { Pop-Location }
    # Restore a useful synthetic history, not just a one-row migration fixture.
    # Deterministic UUIDs are local test identifiers, not application ID generation.
    Invoke-SQL $testDatabase @'
INSERT INTO tickets(id,organization_id,created_by_user_id,reference,title) VALUES('10000000-0000-4000-8000-000000000007','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','RST-2','Second restored ticket');
INSERT INTO tags(id,organization_id,name) VALUES('10000000-0000-4000-8000-000000000008','10000000-0000-4000-8000-000000000002','Restored tag');
INSERT INTO time_entries(id,organization_id,user_id,project_id,description,source_type,status,started_at,ended_at,duration_seconds)
SELECT md5('release-entry-'||n)::uuid,'10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000004','Synthetic restore history',CASE WHEN n%5=0 THEN 'TIMER' ELSE 'MANUAL' END,'STOPPED','2024-01-01'::timestamptz+n*INTERVAL '1 hour','2024-01-01'::timestamptz+n*INTERVAL '1 hour'+INTERVAL '40 minutes',CASE WHEN n%5=0 THEN 1800 ELSE 2400 END FROM generate_series(1,10000) n;
INSERT INTO time_entry_tickets(organization_id,time_entry_id,ticket_id) SELECT te.organization_id,te.id,t.id FROM time_entries te CROSS JOIN tickets t WHERE te.organization_id='10000000-0000-4000-8000-000000000002' ON CONFLICT DO NOTHING;
INSERT INTO time_entry_tags(time_entry_id,tag_id) SELECT id,'10000000-0000-4000-8000-000000000008' FROM time_entries WHERE organization_id='10000000-0000-4000-8000-000000000002';
INSERT INTO timer_events(id,time_entry_id,event_type,occurred_at,sequence_number) SELECT gen_random_uuid(),te.id,event.kind,te.started_at+event.minutes*INTERVAL '1 minute',event.sequence FROM time_entries te CROSS JOIN (VALUES('START',0,1),('PAUSE',10,2),('RESUME',20,3),('STOP',40,4)) event(kind,minutes,sequence) WHERE te.organization_id='10000000-0000-4000-8000-000000000002' AND te.source_type='TIMER';
'@
    $restoreAssertions = $assertions.Replace('(SELECT count(*) FROM time_entries) <> 1 OR (SELECT count(*) FROM tickets) <> 1', '(SELECT count(*) FROM time_entries) <> 10001 OR (SELECT count(*) FROM tickets) <> 2') + @'
DO $$ BEGIN
IF (SELECT sum(duration_seconds) FROM time_entries) <> 22803600 THEN RAISE EXCEPTION 'Restored total duration mismatch'; END IF;
IF (SELECT count(*) FROM time_entry_tickets) <> 20002 OR (SELECT count(*) FROM time_entry_tags) <> 10001 OR (SELECT count(*) FROM timer_events) <> 8000 THEN RAISE EXCEPTION 'Restored associations/events missing'; END IF;
IF EXISTS(SELECT 1 FROM time_entries te WHERE source_type='TIMER' AND (SELECT count(*) FROM timer_events e WHERE e.time_entry_id=te.id) <> 4) THEN RAISE EXCEPTION 'Timer history mismatch'; END IF;
END $$;
'@
    Invoke-SQL $testDatabase $restoreAssertions
    $restoreTimer = [System.Diagnostics.Stopwatch]::StartNew()
    docker exec $PostgresContainer pg_dump -U $PostgresUser -d $testDatabase --format=custom --file=$dumpPath
    Assert-Exit 'Synthetic backup'
    docker exec $PostgresContainer pg_restore -U $PostgresUser -d $restoreDatabase --exit-on-error $dumpPath
    Assert-Exit 'Restore rehearsal'
    Invoke-SQL $restoreDatabase $restoreAssertions
    $restoreTimer.Stop()
    Write-Host "PASS: forward migrations, integration tests, and synthetic backup/restore ($($restoreTimer.Elapsed.TotalSeconds.ToString('F1'))s)."
    Write-Host 'This is local functional evidence, not production-scale RTO or capacity validation.'
} finally {
    $env:DATABASE_URL = $previousDatabaseURL
    foreach ($database in $createdDatabases) {
        if ($database -notmatch '^tt_(release|restore)_[a-f0-9]{32}$') { throw 'Unsafe cleanup target' }
        docker exec $PostgresContainer dropdb -U $PostgresUser --if-exists $database
    }
    if ($createdRole) {
        if ($runtimeRole -notmatch '^tt_runtime_[a-f0-9]{32}$') { throw 'Unsafe role cleanup target' }
        docker exec $PostgresContainer psql -U $PostgresUser -d postgres -v ON_ERROR_STOP=1 -q -c "DROP ROLE $runtimeRole;"
    }
    if ($createdDatabases.Count -gt 0) { docker exec $PostgresContainer rm -f -- $dumpPath }
}
