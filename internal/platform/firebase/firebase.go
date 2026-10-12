package firebase

import (
	"errors"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"

	"app/internal/platform/config"
	"context"
)

type Identity struct {
	UID           string
	Email         string
	EmailVerified bool
	Name          string
}

// Verifier dipakai middleware untuk memverifikasi ID token.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

// UserAdmin dipakai modul yang mengelola akun Firebase (create/delete).
type UserAdmin interface {
	CreateUser(ctx context.Context, email, name string) (uid string, err error)
	DeleteUser(ctx context.Context, uid string) error
}

// Client membungkus Admin SDK Auth; memenuhi Verifier dan UserAdmin.
type Client struct {
	client *auth.Client
}

// New membangun Auth client dari file service account. Dipanggil sekali saat
// startup; hasilnya aman dipakai concurrent. projectID kosong = dibaca otomatis
// dari file (dan itu yang mengunci audience token).
func New(ctx context.Context, cfg *config.Firebase) (*Client, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.ProjectID}, option.WithAuthCredentialsFile(option.ServiceAccount, cfg.CredentialsFile))
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("init firebase auth: %w", err)
	}

	return &Client{client: client}, nil
}

// Verify memverifikasi signature, expiry, issuer, dan audience secara lokal
// (public key Google di-cache), jadi tidak ada network call per request.
// Sengaja TIDAK memakai VerifyIDTokenAndCheckRevoked: itu memanggil server
// Google tiap request. Revocation kita tangani lewat status di DB/Redis.
func (c *Client) Verify(ctx context.Context, idToken string) (Identity, error) {
	token, err := c.client.VerifyIDToken(ctx, idToken)
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

// ErrEmailExists menandai email sudah terdaftar di Firebase.
var ErrEmailExists = errors.New("firebase: email already exists")

// CreateUser mendaftarkan user baru di Firebase (tanpa password; user mengurus
// password sendiri lewat alur Firebase/FE). Mengembalikan UID.
func (c *Client) CreateUser(ctx context.Context, email, name string) (string, error) {
	params := (&auth.UserToCreate{}).Email(email)
	if name != "" {
		params = params.DisplayName(name)
	}
	u, err := c.client.CreateUser(ctx, params)
	if err != nil {
		if auth.IsEmailAlreadyExists(err) {
			return "", ErrEmailExists
		}
		return "", fmt.Errorf("create firebase user: %w", err)
	}
	return u.UID, nil
}

// DeleteUser menghapus user di Firebase. Idempoten: user yang sudah tidak ada
// dianggap sukses.
func (c *Client) DeleteUser(ctx context.Context, uid string) error {
	if err := c.client.DeleteUser(ctx, uid); err != nil {
		if auth.IsUserNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete firebase user: %w", err)
	}
	return nil
}
