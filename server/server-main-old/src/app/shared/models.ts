export type ViewKey = 'overview' | 'users' | 'devices' | 'admins' | 'roles' | 'menus' | 'relays' | 'wire' | 'ice' | 'quality' | 'settings';
export type PagedViewKey = Exclude<ViewKey, 'overview' | 'relays' | 'settings'>;

export type OpsOverview = {
  userCount: number;
  deviceCount: number;
  onlineDeviceCount: number;
  nodeCount: number;
  relayClusterCount: number;
  relayNodeCount: number;
  relayOnlineNodeCount: number;
  defaultAdminSeeded: boolean;
  defaultAdminLoginName?: string;
  defaultAdminRoleBound: boolean;
  securityWarnings?: string[];
};

export type OpsLoginResponse = {
  adminId: string;
  userId: string;
  loginName: string;
  displayName: string;
  accessToken: string;
  expiresIn: number;
};

export type PlanConfig = {
  maxActiveDevices: number;
  relayIngressKbps: number;
  relayEgressKbps: number;
  udpIngressKbps: number;
  udpEgressKbps: number;
};

export type OpsUser = {
  userId: string;
  email: string;
  deviceCount: number;
  nodeCount: number;
  roleCodes?: string[];
  planOverride?: PlanConfig;
};

export type OpsDevice = {
  deviceId: string;
  name: string;
  platform: string;
  status: string;
  userId: string;
  nodeCount: number;
};

export type OpsAdmin = {
  adminId?: string;
  userId: string;
  loginName: string;
  email?: string;
  displayName: string;
  phone?: string;
  title?: string;
  department?: string;
  status: string;
  roleCodes?: string[];
  usingSeedPassword?: boolean;
};

export type OpsRole = {
  roleId: string;
  roleCode: string;
  roleName: string;
  description?: string;
  builtin: boolean;
  menuCodes?: string[];
};

export type OpsMenu = {
  menuId: string;
  menuCode: string;
  menuName: string;
  path?: string;
  parentId?: string;
  sort: number;
  status: string;
};

export type OpsRelayTopology = {
  defaultClusterId: string;
  regions?: Array<{
    regionId: string;
    regionName: string;
    countryName?: string;
    cityName?: string;
    clusterId?: string;
    clusterName?: string;
    endpoints?: Array<{ endpointId: string; transport: string; address: string }>;
  }>;
  nodes?: Array<{
    nodeId: string;
    clusterId: string;
    clusterName: string;
    countryName?: string;
    cityName?: string;
    transport: string;
    address: string;
    priority: number;
    observedRttMs?: number;
    packetLossPpm?: number;
    pathScore?: number;
    heartbeatOnline?: boolean;
    heartbeatLastSeenAt?: number;
    activeSessions?: number;
  }>;
};

export type OpsNetworkQuality = {
  items?: OpsNetworkQualityItem[];
};

export type OpsNetworkQualityItem = {
  healthId: string;
  networkId: string;
  networkName?: string;
  userId?: string;
  userEmail?: string;
  deviceId?: string;
  deviceName?: string;
  nodeId: string;
  peerNodeId?: string;
  pathType: string;
  endpoint?: string;
  derpNodeId?: string;
  observedRttMs?: number;
  packetLossPpm?: number;
  pathScore?: number;
  sampledAtMs?: number;
  updatedAt: number;
};

export type OpsIceServer = {
  serverId: string;
  name?: string;
  provider: string;
  region: string;
  country?: string;
  publicIp?: string;
  udpAddr: string;
  stunPort?: number;
  priority: number;
  weight: number;
  status?: string;
  features?: string[];
  remark?: string;
  createdAt?: number;
  updatedAt?: number;
};

export type OpsIceStats = {
  servers?: OpsIceServerStats[];
};

export type OpsIceServerStats = {
  serverId: string;
  region?: string;
  probeCount: number;
  candidateCount: number;
  p2pSuccessCount: number;
  p2pFailureCount: number;
  avgProbeRttMs?: number;
  successRate: number;
  updatedAt?: number;
};

export type IceServerDraft = {
  serverId: string;
  name: string;
  provider: string;
  region: string;
  country: string;
  publicIp: string;
  udpAddr: string;
  stunPort: number;
  priority: number;
  weight: number;
  status: string;
  featuresText: string;
  remark: string;
};

export type WireNodeBase = {
  regionId: string;
  nodeId: string;
  host: string;
  enabled: boolean;
  healthy: boolean;
  stale: boolean;
  priority: number;
  updatedAtMs: number;
};

export type WireDerpNode = WireNodeBase & {
  name?: string;
  port: number;
};

export type WireRelayNode = WireNodeBase & {
  udpPort: number;
  adminPort?: number;
};

export type WireTicketKeyStatus = {
  signingConfigured: boolean;
  keyRingConfigured: boolean;
  keyRingSize: number;
  rotationReady: boolean;
};

export type WireNodesOpsView = {
  derpNodes: WireDerpNode[];
  relayNodes: WireRelayNode[];
  schedulableDerpCount: number;
  schedulableRelayCount: number;
  staleCount: number;
  ticketKeyRotation: WireTicketKeyStatus;
};

export type WireNodeEvent = {
  eventId: number;
  nodeKind: string;
  regionId: string;
  nodeId: string;
  eventType: string;
  fromEnabled?: boolean;
  toEnabled?: boolean;
  fromHealthy?: boolean;
  toHealthy?: boolean;
  reason?: string;
  createdAtMs: number;
};

export type WireNodeEventList = {
  items: WireNodeEvent[];
  page: number;
  pageSize: number;
  total: number;
};

export type WireNodeEventFilter = {
  nodeKind: string;
  regionId: string;
  nodeId: string;
  eventType: string;
  createdFromMs: string;
  createdToMs: string;
  page: number;
  pageSize: number;
};
