export type AuthResponse = {
  userId: string;
  accessToken: string;
  refreshToken?: string;
  expiresIn: number;
};

export type Device = {
  deviceId: string;
  name: string;
  ownerEmail?: string;
  platform: string;
  machineId?: string;
  status: string;
  currentVirtualIp?: string;
  linkStatus?: string;
  connectivityProtocol?: string;
  joinedAt?: number;
  createdAt?: number;
};

export type Network = {
  networkId: string;
  name: string;
  description?: string;
  defaultSubnetId?: string;
  defaultSubnetCidr?: string;
  joinKeyConfigured?: boolean;
};

export type DNSConfig = {
  servers: string[];
  searchDomains: string[];
};

export type NetworkHome = {
  activeNetwork?: Network;
  ownedNetwork?: Network;
  hasNetwork: boolean;
};

export type NetworkDetail = {
  networkId: string;
  name: string;
  description?: string;
  defaultSubnetId?: string;
  defaultSubnetCidr?: string;
  ownedByCurrentUser: boolean;
  dns: DNSConfig;
  joinKey?: string;
};

export type Subnet = {
  subnetId: string;
  networkId: string;
  name: string;
  cidr: string;
  gatewayIp?: string;
  allocationStartIp?: string;
  allocationEndIp?: string;
  isDefault: boolean;
  status?: string;
};

export type NetworkAssignment = {
  attachmentId: string;
  networkId: string;
  subnetId: string;
  deviceId: string;
  deviceName: string;
  userId: string;
  userEmail: string;
  role: string;
  remark?: string;
  virtualIp?: string;
  status?: string;
};
