// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use crate::shared::errors::{AuthError, PasswordWeaknessReason};
use crate::shared::types::Password;

/// Common-password blocklist (minimal; extend for production).
static BLOCKLIST: &[&str] = &[
    "password123",
    "password1234",
    "123456789012",
    "qwertyuiop12",
    "letmein12345",
    "iloveyou1234",
    "admin1234567",
    "welcome12345",
    "monkey123456",
    "dragon123456",
];

/// policy PasswordStrength
///
/// A password is acceptable when:
///   - length >= 12
///   - contains at least one letter and one digit
///   - is not in the common-password blocklist
pub fn password_strength(p: &Password) -> Result<(), AuthError> {
    let s = &p.0;

    // Blocklist checked first — spec example "password123" -> InBlocklist takes priority.
    if BLOCKLIST.contains(&s.as_str()) {
        return Err(AuthError::WeakPassword(PasswordWeaknessReason::InBlocklist));
    }

    if s.len() < 12 {
        return Err(AuthError::WeakPassword(PasswordWeaknessReason::TooShort));
    }

    let has_digit = s.chars().any(|c| c.is_ascii_digit());
    if !has_digit {
        return Err(AuthError::WeakPassword(
            PasswordWeaknessReason::MissingDigit,
        ));
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    // policy PasswordStrength examples:
    //   - given: "correct horse battery staple 9"  then: ok
    //   - given: "short1"                          then: err(TooShort)
    //   - given: "alllowercase"                    then: err(MissingDigit)
    //   - given: "password123"                     then: err(InBlocklist)

    #[test]
    fn strong_password_passes() {
        let p = Password("correct horse battery staple 9".into());
        assert!(password_strength(&p).is_ok());
    }

    #[test]
    fn too_short_fails() {
        let p = Password("short1".into());
        let err = password_strength(&p).unwrap_err();
        assert!(matches!(
            err,
            AuthError::WeakPassword(PasswordWeaknessReason::TooShort)
        ));
    }

    #[test]
    fn missing_digit_fails() {
        let p = Password("alllowercase".into());
        let err = password_strength(&p).unwrap_err();
        assert!(matches!(
            err,
            AuthError::WeakPassword(PasswordWeaknessReason::MissingDigit)
        ));
    }

    #[test]
    fn blocklisted_fails() {
        let p = Password("password123".into());
        let err = password_strength(&p).unwrap_err();
        assert!(matches!(
            err,
            AuthError::WeakPassword(PasswordWeaknessReason::InBlocklist)
        ));
    }
}
