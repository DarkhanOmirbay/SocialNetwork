# SocialNetwork GRPC Microservices Protobuf
# User-service
# Post-service
tables:
CREATE TABLE users(
id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY ,
email varchar(255) UNIQUE NOT NULL ,
pass_hash bytea NOT NULL
);
CREATE TABLE tokens(
hash bytea PRIMARY KEY ,
user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE ,
expiry TIMESTAMP(0) WITH TIME ZONE NOT NULL
);
CREATE TABLE posts(
id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY ,
user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE ,
text TEXT NOT NULL
);
