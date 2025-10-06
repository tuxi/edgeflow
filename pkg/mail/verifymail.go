package mail

import (
	"errors"

	emailverifier "github.com/AfterShip/email-verifier"
)

type Verifier struct {
	verifier *emailverifier.Verifier
}

func NewVerifier() *Verifier {
	proxyUrl := ""
	if proxyUrl == "" {
		return &Verifier{
			verifier: emailverifier.NewVerifier().EnableSMTPCheck().DisableCatchAllCheck().FromEmail("user@wooveep.net"),
		}
	} else {
		return &Verifier{
			verifier: emailverifier.NewVerifier().Proxy(proxyUrl).EnableSMTPCheck().DisableCatchAllCheck().FromEmail("user@wooveep.net"),
		}
	}

}
func (v *Verifier) VerifierEmail(email string) error {

	ret, err := v.verifier.Verify(email)
	if err != nil {
		// logger.Warnf("verify email address failed, error is: ", err)
		return err
	}
	if !ret.Syntax.Valid {
		// logger.Warnf("email address syntax is invalid")
		return errors.New("email address syntax is invalid")
	}
	if !ret.SMTP.Deliverable {
		return errors.New("email address not deliverable")
	}
	return nil
}
