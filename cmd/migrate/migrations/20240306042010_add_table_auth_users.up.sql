create table if not exists auth.users (
  id UUID primary key default gen_random_uuid(),
  email text not null,
  password text not null,
  created_at timestamp not null default now(),
  updated_at timestamp not null default now()
);