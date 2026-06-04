package biz

import (
	"context"
)

func (s *Store) loadPostgresIPAMLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select id,cidr_block::text,host(base_ip),prefix_length,start_offset,end_offset,generated_capacity,status,extract(epoch from created_at)::bigint from ipam_subnets`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var subnet IPAMSubnet
		if err := rows.Scan(&subnet.SubnetID, &subnet.CIDRBlock, &subnet.BaseIP, &subnet.PrefixLength, &subnet.StartOffset, &subnet.EndOffset, &subnet.GeneratedCapacity, &subnet.Status, &subnet.CreatedAt); err != nil {
			return err
		}
		s.ipamSubnets[subnet.SubnetID] = subnet
		s.nextIPSubnetSeq = maxInt(s.nextIPSubnetSeq, numericIDSuffix(subnet.SubnetID)+1)
		s.nextIPOffset = maxUint32(s.nextIPOffset, subnet.EndOffset+1)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	addressRows, err := s.db.QueryContext(ctx, `select id,subnet_id,host(ip),cidr_block::text,address_offset,coalesce(device_id,''),status,extract(epoch from created_at)::bigint,coalesce(extract(epoch from assigned_at)::bigint,0),coalesce(extract(epoch from released_at)::bigint,0) from global_ip_addresses`)
	if err != nil {
		return err
	}
	defer addressRows.Close()
	for addressRows.Next() {
		var address GlobalIPAddress
		if err := addressRows.Scan(&address.AddressID, &address.SubnetID, &address.IP, &address.CIDRBlock, &address.Offset, &address.DeviceID, &address.Status, &address.CreatedAt, &address.AssignedAt, &address.ReleasedAt); err != nil {
			return err
		}
		s.globalIPs[address.IP] = address
		if device, ok := s.devices[address.DeviceID]; ok {
			device.GlobalIP = address.IP
			s.devices[device.DeviceID] = device
		}
		s.nextIPAddressSeq = maxInt(s.nextIPAddressSeq, numericIDSuffix(address.AddressID)+1)
	}
	return addressRows.Err()
}
