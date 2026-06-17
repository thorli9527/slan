create table users (
  id varchar(128) primary key,
  email varchar(320) not null unique,
  password_hash varchar(255) not null,
  display_name varchar(128),
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table user_sessions (
  id varchar(128) primary key,
  user_id varchar(128) not null references users(id),
  refresh_token_hash varchar(255) not null,
  access_token varchar(255) not null default '',
  client_type varchar(32) not null default 'web',
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);

create index ix_user_sessions_access_token on user_sessions(access_token);

create table audit_events (
  id varchar(128) primary key,
  actor_type varchar(32) not null,
  actor_id varchar(128),
  actor_email varchar(320),
  action varchar(96) not null,
  resource_type varchar(64),
  resource_id varchar(128),
  status varchar(32) not null,
  remote_ip inet,
  details jsonb not null default '{}',
  created_at timestamptz not null default now()
);

create index ix_audit_events_actor on audit_events(actor_type, actor_id, created_at desc);
create index ix_audit_events_action on audit_events(action, status, created_at desc);

create table login_failures (
  key varchar(255) primary key,
  failed_count integer not null,
  first_failed_at timestamptz not null,
  last_failed_at timestamptz not null,
  blocked_until timestamptz
);

create index ix_login_failures_blocked_until on login_failures(blocked_until);

create table console_login_keys (
  login_key varchar(128) primary key,
  user_id varchar(128) not null references users(id),
  device_id varchar(128),
  status varchar(32) not null default 'unused',
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  consumed_at timestamptz
);

create table devices (
  id varchar(128) primary key,
  owner_user_id varchar(128) references users(id),
  device_id varchar(128) not null unique,
  name varchar(128),
  platform varchar(32) not null,
  os_name varchar(64),
  os_version varchar(64),
  alias varchar(128),
  public_key text,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table user_aliases (
  id varchar(128) primary key,
  owner_user_id varchar(128) not null references users(id),
  email varchar(320) not null,
  alias varchar(128) not null,
  updated_at timestamptz not null default now(),
  unique(owner_user_id, email)
);

create table device_owner_change_logs (
  id varchar(128) primary key,
  device_id varchar(128) not null,
  from_user_id varchar(128) references users(id),
  to_user_id varchar(128) not null references users(id),
  reason varchar(64) not null,
  changed_at timestamptz not null default now()
);

create table device_invites (
  id varchar(128) primary key,
  inviter_user_id varchar(128) not null references users(id),
  invite_code varchar(64) not null unique,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  accepted_device_id varchar(128),
  accepted_user_id varchar(128) references users(id),
  accepted_at timestamptz
);

create table device_access_grants (
  id varchar(128) primary key,
  device_id varchar(128) not null references devices(id),
  user_id varchar(128) not null references users(id),
  granted_by varchar(128) references users(id),
  invite_code varchar(64),
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  unique(device_id, user_id)
);

create table ipam_subnets (
  id varchar(128) primary key,
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
  id varchar(128) primary key,
  subnet_id varchar(128) not null references ipam_subnets(id),
  ip inet not null,
  address_offset bigint not null,
  cidr_block cidr not null,
  device_id varchar(128) references devices(id),
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

create table networks (
  id varchar(128) primary key,
  owner_user_id varchar(128) not null references users(id),
  name varchar(128) not null,
  code varchar(64) not null,
  template_key varchar(64),
  intra_group_policy varchar(32) not null default 'allow',
  is_default boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index ux_networks_owner_code on networks(owner_user_id, code);

create table device_bootstrap_keys (
  id varchar(128) primary key,
  key_hash varchar(128) not null unique,
  created_by_user_id varchar(128) not null references users(id),
  network_id varchar(128) not null references networks(id),
  device_alias varchar(128),
  status varchar(32) not null default 'unused',
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  used_at timestamptz,
  used_by_device_id varchar(128),
  revoked_at timestamptz
);

create table network_devices (
  id varchar(128) primary key,
  network_id varchar(128) not null references networks(id),
  device_id varchar(128) not null references devices(id),
  owner_user_id varchar(128) not null references users(id),
  alias varchar(128),
  virtual_ip inet,
  enabled boolean not null default true,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique(network_id, device_id)
);

create unique index ux_network_devices_virtual_ip on network_devices(network_id, virtual_ip) where status <> 'deleted' and virtual_ip is not null;

create table network_dns_zones (
  id varchar(128) primary key,
  network_id varchar(128) not null references networks(id),
  zone_name varchar(255) not null,
  expose_global boolean not null default false,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now(),
  unique(network_id, zone_name)
);

create table network_dns_records (
  id varchar(128) primary key,
  zone_id varchar(128) not null references network_dns_zones(id),
  network_id varchar(128) not null references networks(id),
  name varchar(128) not null,
  fqdn varchar(255) not null unique,
  record_type varchar(16) not null default 'A',
  target_device_id varchar(128),
  target_ip inet,
  cname varchar(255),
  port varchar(16),
  ttl integer not null default 60,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now()
);

create table public_domain_mappings (
  id varchar(128) primary key,
  network_id varchar(128) not null references networks(id),
  alias varchar(128) not null,
  public_domain varchar(255) not null unique,
  source_record varchar(255),
  device_id varchar(128) not null,
  protocol varchar(16) not null default 'HTTP',
  port varchar(16) not null,
  external_port varchar(16) not null,
  status varchar(32) not null default 'enabled',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique(network_id, alias)
);

create table security_groups (
  id varchar(128) primary key,
  network_id varchar(128) not null references networks(id),
  name varchar(128) not null,
  description text,
  status varchar(32) not null default 'active',
  created_at timestamptz not null default now()
);

create table security_group_rules (
  id varchar(128) primary key,
  security_group_id varchar(128) not null references security_groups(id),
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
  device_id varchar(128) primary key references devices(id),
  heartbeat_online boolean not null default false,
  network_enabled boolean not null default false,
  device_enabled boolean not null default true,
  rx_bytes_total bigint not null default 0,
  tx_bytes_total bigint not null default 0,
  last_seen_at timestamptz,
  last_report_at timestamptz
);

create table device_groups (
  id varchar(128) primary key,
  user_id varchar(128) not null references users(id),
  name varchar(128) not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique(user_id, name)
);

create table device_group_members (
  group_id varchar(128) not null references device_groups(id) on delete cascade,
  device_id varchar(128) not null references devices(id) on delete cascade,
  added_at timestamptz not null default now(),
  primary key(group_id, device_id)
);

create index ix_device_group_members_device on device_group_members(device_id);

create table device_sessions (
  id varchar(128) primary key,
  device_id varchar(128) not null references devices(id),
  user_id varchar(128) not null references users(id),
  device_token varchar(255) not null unique,
  device_token_expires_at timestamptz not null,
  device_refresh_token varchar(255) not null,
  active_network_ids jsonb not null default '[]',
  state varchar(32) not null default 'active',
  registered_at timestamptz not null,
  last_renewed_at timestamptz not null
);

create table network_config_versions (
  id varchar(128) primary key,
  network_id varchar(128) not null references networks(id),
  device_id varchar(128) not null,
  config_version bigint not null,
  config_hash varchar(128) not null,
  pushed_at timestamptz not null default now(),
  applied_at timestamptz,
  status varchar(32) not null default 'pushed'
);

create table mqtt_control_deliveries (
  id varchar(128) primary key,
  delivery_id varchar(128) not null,
  device_id varchar(128) not null references devices(id),
  message_type varchar(64) not null,
  task_id varchar(255),
  action varchar(64),
  schema_version integer not null default 1,
  status varchar(32) not null default 'pending',
  attempt_count integer not null default 0,
  last_error text,
  payload jsonb,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  published_at timestamptz,
  acked_at timestamptz,
  processed_at timestamptz,
  updated_at timestamptz not null default now(),
  unique(device_id, delivery_id)
);

create index ix_mqtt_control_deliveries_retry on mqtt_control_deliveries(status, expires_at, updated_at) where status in ('pending', 'published', 'retrying');
create index ix_mqtt_control_deliveries_device_status on mqtt_control_deliveries(device_id, status, updated_at);
