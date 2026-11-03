package auth

import "unicode"

const passwordRequirements = "Password must contain at least 8 characters, one uppercase letter, and one number."

func validNewPassword(password string) bool {
	length := 0
	hasUppercase := false
	hasNumber := false
	for _, character := range password {
		length++
		hasUppercase = hasUppercase || unicode.IsUpper(character)
		hasNumber = hasNumber || unicode.IsDigit(character)
	}
	return length >= 8 && hasUppercase && hasNumber
}
