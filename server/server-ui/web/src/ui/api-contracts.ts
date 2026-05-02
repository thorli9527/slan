export type AuthResponse = {
  userId: string;
  email?: string;
  accessToken: string;
  refreshToken?: string;
  expiresIn: number;
};

export type ConsoleLoginKeyResponse = {
  loginKey: string;
  expiresIn: number;
};

export type ErrorResponse = {
  code: string;
  message: string;
};

export type PublicSystemConfig = {
  allowRegistration: boolean;
};

export type CompleteAuthCallbackRequest = {
  accessToken: string;
  userId: string;
  refreshToken?: string;
  expiresIn: number;
  deviceId?: string;
  userLabel?: string;
  action?: string;
};

export type ChangePasswordRequest = {
  currentPassword: string;
  newPassword: string;
};

export type AuthCallbackStatusResponse = {
  callbackId: string;
  ready: boolean;
  payload?: CompleteAuthCallbackRequest;
};

export type MQTTCredential = {
  brokerUrl: string;
  clientId: string;
  username: string;
  password: string;
  topicPrefix: string;
  expiresAt?: number;
};

export type DeviceNetworkState = {
  deviceId: string;
  networkId: string;
  controlReachable: boolean;
  networkOnline: boolean;
  tunnelUp: boolean;
  lastProbeOk: boolean;
  virtualIp?: string;
  lastSeenAt: number;
  updatedAt: number;
};

export type Device = {
  deviceId: string;
  name: string;
  ownerEmail?: string;
  platform: string;
  status: string;
  currentVirtualIp?: string;
  linkStatus?: string;
  connectivityProtocol?: string;
  joinedAt?: number;
  membershipStatus?: string;
  networkRole?: string;
  createdAt?: number;
  publicKey?: string;
  networkIds?: string[];
  mqtt?: MQTTCredential;
  networkState?: DeviceNetworkState;
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
  wildcards?: string[];
};

export type UpdateNetworkDNSRequest = {
  servers?: string[];
  searchDomains?: string[];
  wildcards?: string[];
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
  joinKeyConfigured?: boolean;
  ownedByCurrentUser: boolean;
  dns: DNSConfig;
  joinKey?: string;
  subnets?: Subnet[];
  members?: NetworkMember[];
};

export type Subnet = {
  subnetId: string;
  networkId: string;
  name: string;
  cidr: string;
  remark?: string;
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
  devicePlatform?: string;
  deviceVersion?: string;
  connectionType?: string;
  runtimeControlReachable?: boolean;
  runtimeNetworkOnline?: boolean;
  runtimeTunnelUp?: boolean;
  runtimeVirtualIp?: string;
  runtimeLastSeenAt?: number;
  runtimeStateFresh?: boolean;
  runtimeHeartbeatOnline?: boolean;
  runtimeNetworkEnabled?: boolean;
  runtimeDeviceDisabled?: boolean;
  runtimeIpApplied?: boolean;
  userId: string;
  userEmail: string;
  role: string;
  remark?: string;
  virtualIp?: string;
  status?: string;
};

export type NetworkMember = {
  memberId: string;
  networkId: string;
  deviceId: string;
  role: string;
  createdAt?: number;
  status?: string;
};

export type UpdateNetworkMemberStatus = 'active' | 'rejected';

export type SubnetAttachment = {
  attachmentId: string;
  networkId: string;
  subnetId: string;
  deviceId: string;
  virtualIp?: string;
  remark?: string;
  status?: string;
};

export type NetworkJoinResult = {
  member: NetworkMember;
  attachment: SubnetAttachment;
};

export type NetworkJoinByOwnerEmailResult = {
  network: Network;
  member: NetworkMember;
  attachment: SubnetAttachment;
};

export type ControlPlaneConfig = {
  wsUrl: string;
  heartbeatSeconds: number;
};

export type RelayNode = {
  nodeId: string;
  transport: string;
  address: string;
  priority: number;
  tags?: string[];
};

export type RelayCluster = {
  clusterId: string;
  clusterName: string;
  nodes?: RelayNode[];
};

export type RelayCity = {
  cityCode: string;
  cityName: string;
  clusters?: RelayCluster[];
};

export type RelayCountry = {
  countryCode: string;
  countryName: string;
  cities?: RelayCity[];
};

export type RelayConfig = {
  defaultClusterId: string;
  countries?: RelayCountry[];
};

export type DeviceBootstrap = {
  device: Device;
  attachments: {
    networkId: string;
    deviceId: string;
    virtualIp?: string;
  }[];
};

export type BootstrapResponse = {
  controlSessionId?: string;
  sessionToken?: string;
  device: DeviceBootstrap;
  networks: NetworkDetail[];
  controlPlane: ControlPlaneConfig;
  stunServers: string[];
  relay: RelayConfig;
  derpMap: DerpMap;
  networkMap: NetworkMap;
};

export type RelayTicket = {
  ticketId: string;
  networkId: string;
  sessionId: string;
  srcNodeId: string;
  dstNodeId: string;
  derpClusterId?: string;
  countryCode?: string;
  cityCode?: string;
  allowedDerpNodeIds?: string[];
  relayUrl: string;
  expiresAt: string;
  sessionKey?: string;
  signature: string;
};

export type DerpNode = {
  nodeId: string;
  host: string;
  port: number;
  transport: string;
  priority: number;
  tags?: string[];
};

export type DerpCluster = {
  clusterId: string;
  clusterName?: string;
  regionId: string;
  regionName: string;
  countryCode?: string;
  countryName?: string;
  cityCode?: string;
  cityName?: string;
  recommendedFanout: number;
  nodes?: DerpNode[];
};

export type DerpMap = {
  probeIntervalSeconds: number;
  clusters?: DerpCluster[];
};

export type Endpoint = {
  type: string;
  address: string;
  updatedAt: number;
};

export type Peer = {
  nodeId: string;
  deviceId: string;
  publicKey: string;
  status: string;
  relayAllowed: boolean;
  virtualIps?: string[];
  endpoints?: Endpoint[];
  allowedRoutes?: string[];
};

export type Route = {
  cidr: string;
  viaNodeId: string;
  metric?: string;
};

export type RelayEndpoint = {
  endpointId: string;
  transport: string;
  address: string;
};

export type RelayRegion = {
  regionId: string;
  regionName: string;
  countryCode?: string;
  countryName?: string;
  cityCode?: string;
  cityName?: string;
  clusterId?: string;
  clusterName?: string;
  endpoints?: RelayEndpoint[];
};

export type NetworkMap = {
  selfUserId: string;
  selfDeviceId: string;
  selfNodeId: string;
  networkId: string;
  revision: number;
  heartbeatSeconds: number;
  stunServers?: string[];
  peers?: Peer[];
  routes?: Route[];
  relayRegions?: RelayRegion[];
  dns: DNSConfig;
  mtu?: number;
};

export type PlanStatus = {
  planName: string;
  freeDeviceLimit: number;
  maxActiveDevices: number;
  relayBandwidthLimitKbps?: number;
  p2pUnlimited?: boolean;
  dnsAvailable: boolean;
};
