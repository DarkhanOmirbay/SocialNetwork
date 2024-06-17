package main

import (
	"context"
	_go "darkhan/gen/go"
	"database/sql"
	"errors"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
)

func ConnectPostgres() (*sql.DB, error) {
	connstring := "user=postgres dbname=project password=703905 host=localhost port=5432 sslmode=disable"
	db, err := sql.Open("postgres", connstring)
	if err != nil {
		return nil, err
	}
	return db, nil
}
func main() {
	db, err := ConnectPostgres()
	if err != nil {
		log.Panic(err)
	}
	log.Println("Connected to postgres database")
	l, err := net.Listen("tcp", ":44044")
	if err != nil {
		log.Fatalf("Failed to listen:%v", err)
	}
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}
	defer conn.Close()
	userClient := _go.NewUserClient(conn)
	s := grpc.NewServer()

	_go.RegisterPostServer(s, &PostServer{userClient: userClient, db: db})
	log.Println("Server is running on port :44044")
	err = s.Serve(l)
	if err != nil {
		log.Fatalf("failed to serve:%v", err)

	}
}

type PostServer struct {
	_go.UnimplementedPostServer
	userClient _go.UserClient
	db         *sql.DB
}

func (p *PostServer) CreatePost(ctx context.Context, request *_go.CreatePostRequest) (*_go.CreatePostResponse, error) {
	if request.Token == "" {
		return nil, errors.New("token is required")
	}
	if request.Text == "" {
		return nil, errors.New("text is required")
	}
	userId, err := p.GetIdByToken(ctx, request.Token)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	row := p.db.QueryRow("INSERT INTO posts(user_id,text) VALUES ($1,$2) RETURNING id", userId, request.Text)
	var postId int64
	err = row.Scan(&postId)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, status.Error(codes.Internal, "sql no rows")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &_go.CreatePostResponse{PostId: postId}, nil
}
func (p *PostServer) ReadPost(ctx context.Context, request *_go.ReadPostRequest) (*_go.ReadPostResponse, error) {
	if request.Token == "" {
		return nil, errors.New("token is required")
	}
	if request.PostId == 0 || request.PostId < 0 {
		return nil, errors.New("id is required")
	}
	userID, err := p.GetIdByToken(ctx, request.Token)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	row := p.db.QueryRow("SELECT text FROM posts WHERE id=$1 AND user_id=$2", request.PostId, userID)

	post := &_go.PostInfo{
		PostId: request.PostId,
		UserId: userID,
	}
	err = row.Scan(&post.Text)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, status.Error(codes.Internal, "post not found")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}

	}
	return &_go.ReadPostResponse{Post: post}, nil
}

func (p *PostServer) UpdatePost(ctx context.Context, request *_go.UpdatePostRequest) (*_go.UpdatePostResponse, error) {
	if request.PostId <= 0 {
		return nil, errors.New("post id is required")
	}
	if request.Token == "" {
		return nil, errors.New("token is required")
	}
	if request.NewText == "" {
		return nil, errors.New("text is required")
	}
	userID, err := p.GetIdByToken(ctx, request.Token)
	if err != nil {
		return nil, errors.New("error token")
	}
	_, err = p.db.Exec("UPDATE posts SET text=$1 WHERE id=$2 AND user_id=$3", request.NewText, request.PostId, userID)
	if err != nil {
		return nil, errors.New("error updating post")
	}
	log.Println("Post updated")
	return &_go.UpdatePostResponse{Msg: "Post updated"}, nil
}
func (p *PostServer) DeletePost(ctx context.Context, request *_go.DeletePostRequest) (*_go.DeletePostResponse, error) {
	if request.Token == "" {
		return nil, errors.New("token is required")
	}
	if request.PostId <= 0 {
		return nil, errors.New("post id is required")
	}
	userID, err := p.GetIdByToken(ctx, request.Token)
	if err != nil {
		return nil, errors.New("error token")
	}
	_, err = p.db.Exec("DELETE FROM posts WHERE id=$1 AND user_id=$2", request.PostId, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, errors.New("post not found")
		default:
			return nil, errors.New("error deleting post")
		}

	}
	return &_go.DeletePostResponse{Msg: "Post deleted"}, nil
}
func (p *PostServer) GetIdByToken(ctx context.Context, token string) (userId int64, err error) {
	resp, err := p.userClient.CheckToken(ctx, &_go.TokenRequest{Token: token})
	if err != nil || !resp.IsAuthenticated {
		return 0, err
	}
	return resp.UserId, nil
}
