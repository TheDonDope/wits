CREATE TABLE IF NOT EXISTS USER (
  id text not null primary key,
  email varchar(255),
  password varchar(255),
  name varchar(255)
);