import type { Page } from '../types/domain'

export const pages: Record<Page, { icon: string; title: string; description: string }> = {
  tracker: {
    icon: '◷',
    title: 'Tracker',
    description: 'Capture focused work in the right workspace.',
  },
  entries: {
    icon: '☷',
    title: 'Entries',
    description: 'Review completed work in this workspace.',
  },
  reports: {
    icon: '▤',
    title: 'Reports',
    description: 'Analyze completed work across a selected date range.',
  },
  projects: {
    icon: '▣',
    title: 'Projects',
    description: 'Manage projects and the Members assigned to them.',
  },
  tickets: {
    icon: '◫',
    title: 'Tickets',
    description: 'Manage ticket references used to categorize work.',
  },
  tags: { icon: '⌘', title: 'Tags', description: 'Organize work with flexible labels.' },
  team: { icon: '◉', title: 'Team', description: 'Invite people and manage organization access.' },
  settings: {
    icon: '⚙',
    title: 'Settings',
    description: 'Manage your profile and workspace preferences.',
  },
}
