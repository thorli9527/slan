drop index if exists ux_networks_owner_code;

alter table networks
  drop column if exists status;

create unique index if not exists ux_networks_owner_code on networks(owner_user_id, code);
