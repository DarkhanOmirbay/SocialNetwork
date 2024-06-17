package main

import (
	"context"
	"crypto/sha256"
	_go "darkhan/gen/go"
	"database/sql"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"time"
)

func main() {
	db, err := ConnectPostgresDB()
	if err != nil {
		log.Panic(err)
	}
	log.Println("Connected to postgres database")
	UserServer := New(_go.UnimplementedUserServer{}, db)
	l, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen:%v", err)
	}
	s := grpc.NewServer()
	_go.RegisterUserServer(s, UserServer)
	log.Println("Server is running on port:50051")
	if err := s.Serve(l); err != nil {
		log.Fatalf("failed to serve:%v", err)
	}
}
func (s *UserServer) Register(ctx context.Context, req *_go.RegisterRequest) (*_go.RegisterResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	PassHash, err := bcrypt.GenerateFromPassword([]byte(req.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		log.Println("failed to generate password hash", err)
		return nil, status.Error(codes.Internal, "failed to generate password hash")
	}
	var UserID int64
	err = s.db.QueryRow("INSERT INTO users(email,pass_hash) VALUES($1,$2) RETURNING id", req.GetEmail(), PassHash).Scan(&UserID)
	if err != nil {
		log.Println(err)
		return nil, status.Error(codes.Internal, "failed to insert user")
	}
	log.Println("user registered")
	return &_go.RegisterResponse{Id: UserID}, nil

}

func (s *UserServer) Login(ctx context.Context, req *_go.LoginRequest) (*_go.LoginResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	var User User
	err := s.db.QueryRow("SELECT id,email,pass_hash FROM users WHERE email=$1", req.GetEmail()).Scan(&User.ID, &User.Email, &User.Password.Hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.Internal, "user not found")
		}
		log.Println(err)
		return nil, status.Error(codes.Internal, "failed to get user by email")
	}
	err = bcrypt.CompareHashAndPassword(User.Password.Hash, []byte(req.GetPassword()))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid password")
	}
	log.Println("user logged in successfully")
	token, err := NewToken(User, time.Hour)
	if err != nil {
		return nil, status.Error(codes.Internal, "err token create new token")
	}
	tokenHash := sha256.Sum256([]byte(token))
	_, err = s.db.Exec("INSERT INTO tokens(hash,user_id,expiry) VALUES($1,$2,$3)", tokenHash[:], User.ID, time.Now().Add(time.Hour))
	if err != nil {
		return nil, status.Error(codes.Internal, "err saving token")
	}
	return &_go.LoginResponse{Token: token}, nil
}
func (s *UserServer) CheckToken(ctx context.Context, req *_go.TokenRequest) (*_go.TokenResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}
	tokenHash := sha256.Sum256([]byte(req.Token))
	row := s.db.QueryRowContext(ctx, "SELECT id,email,pass_hash FROM users INNER JOIN tokens t ON users.id=t.user_id WHERE t.hash=$1 AND t.expiry>$2", tokenHash[:], time.Now())
	var user User
	err := row.Scan(&user.ID, &user.Email, &user.Password.Hash)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, status.Error(codes.Internal, "sql no rows")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	log.Println("token checked")
	return &_go.TokenResponse{UserId: user.ID, IsAuthenticated: true}, nil
}

type User struct {
	ID       int64
	Email    string
	Password Password
}
type Password struct {
	PlainText *string
	Hash      []byte
}

type UserServer struct {
	_go.UnimplementedUserServer
	db *sql.DB
}

func New(server _go.UnimplementedUserServer, db *sql.DB) *UserServer {
	return &UserServer{server, db}
}
func ConnectPostgresDB() (*sql.DB, error) {
	connstring := "user=postgres dbname=project password=703905 host=localhost port=5432 sslmode=disable"
	db, err := sql.Open("postgres", connstring)
	if err != nil {
		return nil, err
	}
	return db, nil
}
func NewToken(user User, duration time.Duration) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["uid"] = user.ID
	claims["email"] = user.Email
	claims["exp"] = time.Now().Add(duration).Unix()

	tokenString, err := token.SignedString([]byte("secret"))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
