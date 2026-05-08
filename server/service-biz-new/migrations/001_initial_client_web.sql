create table users (
  id uuid primary key,
  email varchar(320) not null unique,
  password_hash varchar(255) not null,
  display_name varchar(128),
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table user_sessions (
  id uuid primary key,
  user_id uuid not null references users(id),
  refresh_token_hash varchar(255) not null,
  client_type varchar(32) not null default 'web',
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);

create table devices (
  id uuid primary key,
  device_id varchar(128) not null unique,
  platform varchar(32) not null,
  os_name varchar(64),
  os_version varchar(64),
  alias varchar(128),
  public_key text,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table device_owners (
  id uuid primary key,
  device_id uuid not null references devices(id),
  user_id uuid not null references users(id),
  status varchar(32) not null default 'active',
  bound_at timestamptz not null default now(),
  unbound_at timestamptz
);

create unique index ux_device_active_owner on device_owners(device_id) where status = 'active';

create table device_owner_change_logs (
  id uuid primary key,
  device_id uuid not null references devices(id),
  from_user_id uuid references users(id),
  to_user_id uuid not null references users(id),
  reason varchar(64) not null,
  changed_at timestamptz not null default now()
);

create table ipam_subnets (
  id uuid primary key,
  cidr_block cidr not null unique,
  base_ip inet not null,
  prefix_length integer not null,
  start_offset bigint not null,
  end_offset bigint not null,
  generated_capacity integer not null default 0,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now()
);

create unique index ux_ipam_subnets_offset on ipam_subnets(start_offset, end_offset);

create table global_ip_addresses (
  id uuid primary key,
  subnet_id uuid not null references ipam_subnets(id),
  ip inet not null,
  address_offset bigint not null,
  cidr_block cidr not null,
  device_id uuid references devices(id),
  status varchar(32) not null default 'available',
  created_at timestamptz not null default now(),
  assigned_at timestamptz,
  released_at timestamptz
);

create unique index ux_global_ip_address on global_ip_addresses(ip);
create unique index ux_global_ip_offset on global_ip_addresses(address_offset);
create unique index ux_device_active_ip on global_ip_addresses(device_id) where status = 'assigned';
create index ix_global_ip_pool_available on global_ip_addresses(subnet_id, address_offset) where status = 'available';

-- IPAM generation policy:
-- 1. The global pool starts from 10.0.0.0/8.
-- 2. Addresses are generated in /20 subnet batches so each physical table/partition can map to one subnet.
-- 3. Each batch skips the subnet network and broadcast addresses, then inserts rows with status='available'.
-- 4. Device registration atomically claims the lowest available address and marks it status='assigned'.
-- 5. When available rows fall below 1000, generate the next /20 subnet batch.
-- In production Postgres, create child partitions/tables per ipam_subnets.cidr_block, for example:
--   global_ip_addresses_10_0_0_0_20
--   global_ip_addresses_10_0_16_0_20
-- The service layer should route inserts by subnet_id/address_offset.

create table workspaces (
  id uuid primary key,
  owner_user_id uuid not null references users(id),
  name varchar(128) not null,
  code varchar(64) not null,
  template_key varchar(64),
  status varchar(32) not null default 'enabled',
  is_default boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index ux_workspaces_owner_code on workspaces(owner_user_id, code) where status <> 'deleted';

create table workspace_members (
  id uuid primary key,
  workspace_id uuid not null references workspaces(id),
  user_id uuid not null references users(id),
  role varchar(32) not null default 'member',
  status varchar(32) not null default 'active',
  joined_at timestamptz not null default now(),
  unique(workspace_id, user_id)
);

create table workspace_invites (
  id uuid primary key,
  workspace_id uuid not null references workspaces(id),
  inviter_user_id uuid not null references users(id),
  invitee_user_id uuid references users(id),
  invitee_email varchar(320) not null,
  role varchar(32) not null default 'member',
  status varchar(32) not null default 'pending',
  token_hash varchar(255),
  expires_at timestamptz,
  created_at timestamptz not null default now(),
  handled_at timestamptz
);

create table workspace_device_invites (
  id uuid primary key,
  workspace_id uuid not null references workspaces(id),
  inviter_user_id uuid references users(id),
  invite_code varchar(64) not null unique,
  status varchar(32) not null default 'pending',
  expires_at timestamptz not null,
  created_at timestamptz not null default now(),
  accepted_device_id uuid references devices(id),
  accepted_user_id uuid references users(id),
  accepted_at timestamptz
);

create table workspace_devices (
  id uuid primary key,
  workspace_id uuid not null references workspaces(id),
  device_id uuid not null references devices(id),
  owner_user_id uuid not null references users(id),
  alias varchar(128),
  enabled boolean not null default true,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique(workspace_id, device_id)
);

create table workspace_dns_zones (
  id uuid primary key,
  workspace_id uuid not null references workspaces(id),
  zone_name varchar(255) not null,
  expose_global boolean not null default false,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  unique(workspace_id, zone_name)
);

create table workspace_dns_records (
  id uuid primary key,
  zone_id uuid not null references workspace_dns_zones(id),
  workspace_id uuid not null references workspaces(id),
  name varchar(128) not null,
  fqdn varchar(255) not null unique,
  record_type varchar(16) not null default 'A',
  target_device_id uuid references devices(id),
  target_ip inet,
  cname varchar(255),
  ttl integer not null default 60,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now()
);

create table security_groups (
  id uuid primary key,
  workspace_id uuid not null references workspaces(id),
  name varchar(128) not null,
  description text,
  default_policy varchar(16) not null default 'deny',
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now()
);

create table security_group_devices (
  id uuid primary key,
  security_group_id uuid not null references security_groups(id),
  device_id uuid not null references devices(id),
  unique(security_group_id, device_id)
);

create table security_group_rules (
  id uuid primary key,
  security_group_id uuid not null references security_groups(id),
  direction varchar(16) not null,
  priority integer not null default 100,
  action varchar(16) not null,
  protocol varchar(16) not null default 'all',
  port_from integer,
  port_to integer,
  peer_type varchar(32) not null,
  peer_value varchar(255) not null,
  description text,
  enabled boolean not null default true,
  created_at timestamptz not null default now()
);

create table device_runtime_status (
  device_id uuid primary key references devices(id),
  heartbeat_online boolean not null default false,
  network_enabled boolean not null default false,
  device_enabled boolean not null default true,
  rx_bytes_total bigint not null default 0,
  tx_bytes_total bigint not null default 0,
  last_seen_at timestamptz,
  last_report_at timestamptz
);

create table network_config_versions (
  id uuid primary key,
  workspace_id uuid not null references workspaces(id),
  device_id uuid not null references devices(id),
  config_version bigint not null,
  config_hash varchar(128) not null,
  pushed_at timestamptz not null default now(),
  applied_at timestamptz,
  status varchar(32) not null default 'pushed'
);
