alter table networks
  add column if not exists intra_group_policy varchar(32) not null default 'allow';

do $$
begin
  if exists (
    select 1
    from information_schema.columns
    where table_name = 'security_groups'
      and column_name = 'intra_group_policy'
  ) then
    update networks n
    set intra_group_policy = 'deny'
    where exists (
      select 1
      from security_groups sg
      where sg.network_id = n.id
        and lower(coalesce(sg.intra_group_policy, 'allow')) in ('deny', 'isolated')
    );
  end if;
end $$;

update networks
  set intra_group_policy = 'allow'
  where intra_group_policy is null or intra_group_policy = '';

alter table security_groups
  drop column if exists intra_group_policy;
