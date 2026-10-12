package firebase

import (
	"app/internal/platform/config"
	"context"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"fmt"
	"google.golang.org/api/option"
)

type Identity struct {
	UID           string
	Email         string
	EmailVerified bool
	Name          string
}

type Verifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

type verifier struct {
	client *auth.Client
}

// New membangun Auth client dari file service account. Dipanggil sekali saat
// startup; hasilnya aman dipakai concurrent. projectID kosong = dibaca otomatis
// dari file (dan itu yang mengunci audience token).
func New(ctx context.Context, cfg *config.Firebase) (Verifier, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.ProjectID}, option.WithAuthCredentialsFile(option.ServiceAccount, cfg.CredentialsFile))
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("init firebase auth: %w", err)
	}

	return &verifier{client: client}, nil
}

// Verify memverifikasi signature, expiry, issuer, dan audience secara lokal
// (public key Google di-cache), jadi tidak ada network call per request.
// Sengaja TIDAK memakai VerifyIDTokenAndCheckRevoked: itu memanggil server
// Google tiap request. Revocation kita tangani lewat status di DB/Redis.
func (v *verifier) Verify(ctx context.Context, idToken string) (Identity, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify idtoken: %w", err)
	}
	email, _ := token.Claims["email"].(string)
	name, _ := token.Claims["name"].(string)
	verified, _ := token.Claims["email_verified"].(bool)
	return Identity{
		UID:           token.UID,
		Email:         email,
		EmailVerified: verified,
		Name:          name,
	}, nil
}
