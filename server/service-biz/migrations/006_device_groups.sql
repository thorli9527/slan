create table if not exists device_groups (
  id varchar(128) primary key,
  user_id varchar(128) not null references users(id),
  name varchar(128) not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique(user_id, name)
);

create table if not exists device_group_members (
  group_id varchar(128) not null references device_groups(id) on delete cascade,
  device_id varchar(128) not null references devices(id) on delete cascade,
  added_at timestamptz not null default now(),
  primary key(group_id, device_id)
);

create index if not exists ix_device_group_members_device on device_group_members(device_id);
