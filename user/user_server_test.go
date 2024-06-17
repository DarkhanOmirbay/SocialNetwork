package main

import (
	"context"
	"crypto/sha256"
	_go "darkhan/gen/go"
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func TestNewToken(t *testing.T) {
	plainTextPassword := "123"
	user := User{
		ID:    1,
		Email: "Darkhan",
		Password: Password{
			PlainText: &plainTextPassword,
			Hash:      []byte("klgadagkdllfadka"),
		},
	}

	duration := time.Hour
	tokenString, err := NewToken(user, duration)
	if err != nil {
		t.Fatalf("Expected no error, but got %v", err)
	}

	// Parse the token to verify its claims
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	t.Logf("token: %v", token)
	if err != nil {
		t.Fatalf("Expected no error in parsing token, but got %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		t.Fatalf("Expected valid token, but got invalid")
	}
	t.Logf("claims %v", claims)
	if claims["uid"] != float64(user.ID) {
		t.Errorf("Expected user ID %d, but got %v", user.ID, claims["uid"])
	}

	if claims["email"] != user.Email {
		t.Errorf("Expected email %s, but got %v", user.Email, claims["email"])
	}

	exp := time.Unix(int64(claims["exp"].(float64)), 0)
	if !exp.After(time.Now()) {
		t.Errorf("Expected expiration time to be in the future, but got %v", exp)
	}
}
func TestCheckToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	server := &UserServer{db: db}
	ctx := context.Background()

	tests := []struct {
		name       string
		token      string
		prepare    func()
		wantErr    error
		wantResult *_go.TokenResponse
	}{
		{
			name:  "valid token",
			token: "valid_token",
			prepare: func() {
				tokenHash := sha256.Sum256([]byte("valid_token"))
				rows := sqlmock.NewRows([]string{"id", "email", "pass_hash"}).
					AddRow(1, "test@example.com", []byte("hashed_password"))
				mock.ExpectQuery("SELECT id,email,pass_hash FROM users INNER JOIN tokens t ON users.id=t.user_id WHERE t.hash=\\$1 AND t.expiry>\\$2").
					WithArgs(tokenHash[:], sqlmock.AnyArg()).
					WillReturnRows(rows)
			},
			wantErr: nil,
			wantResult: &_go.TokenResponse{
				UserId:          1,
				IsAuthenticated: true,
			},
		},
		{
			name:  "invalid token",
			token: "invalid_token",
			prepare: func() {
				tokenHash := sha256.Sum256([]byte("invalid_token"))
				mock.ExpectQuery("SELECT id,email,pass_hash FROM users INNER JOIN tokens t ON users.id=t.user_id WHERE t.hash=\\$1 AND t.expiry>\\$2").
					WithArgs(tokenHash[:], sqlmock.AnyArg()).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr:    status.Error(codes.Internal, "sql no rows"),
			wantResult: nil,
		},
		{
			name:       "missing token",
			token:      "",
			prepare:    func() {},
			wantErr:    status.Error(codes.InvalidArgument, "token is required"),
			wantResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			req := &_go.TokenRequest{Token: tt.token}
			res, err := server.CheckToken(ctx, req)
			if tt.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, tt.wantErr, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantResult, res)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
