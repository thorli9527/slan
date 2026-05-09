import { CommonModule } from '@angular/common';
import { Component, ViewEncapsulation } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CustomersPageComponent } from './features/customers/customers-page.component';
import { OperatorsPageComponent } from './features/operators/operators-page.component';
import { OrdersPageComponent } from './features/orders/orders-page.component';
import { OverviewPageComponent } from './features/overview/overview-page.component';
import { ProductsPageComponent } from './features/products/products-page.component';
import { RelayNodesPageComponent } from './features/relay-nodes/relay-nodes-page.component';
import { RenewalsPageComponent } from './features/renewals/renewals-page.component';

type NavId = 'overview' | 'operators' | 'relayNodes' | 'customers' | 'products' | 'orders' | 'renewals';

type OperatorUser = {
  operatorId: string;
  name: string;
  email: string;
  role: 'owner' | 'admin' | 'ops' | 'finance';
  status: 'active' | 'disabled';
  lastLoginAt: string;
};

type RelayNode = {
  nodeId: string;
  name: string;
  region: string;
  transport: 'relay_udp' | 'derp_tcp_tls_443';
  publicAddr: string;
  maxBandwidthMbps: number;
  monthlyTrafficGb: number;
  usedTrafficGb: number;
  maxSessions: number;
  activeSessions: number;
  status: 'active' | 'maintenance' | 'disabled';
  health: 'healthy' | 'warning' | 'down';
};

type CustomerPlan = {
  code: 'free' | 'pro' | 'enterprise';
  name: string;
  ownDeviceLimit: number;
  invitedDeviceLimit: number;
  totalDeviceLimit: number;
  relayMonthlyGb: number;
  relayBandwidthMbps: number;
  relayThrottleMbps: number;
  p2pUnlimited: boolean;
  customDomain: boolean;
  acl: boolean;
  dedicatedRelay: boolean;
  auditLog: boolean;
  apiAccess: boolean;
  monthlyPrice: number;
  yearlyPrice: number;
};

type Product = {
  productId: string;
  name: string;
  type: 'plan' | 'traffic_pack' | 'enterprise';
  planCode?: CustomerPlan['code'];
  period: 'monthly' | 'yearly' | 'one_time' | 'contract';
  validDays: number;
  relayTrafficGb: number;
  relayBandwidthMbps: number;
  listPrice: number;
  salePrice: number;
  currency: 'CNY';
  autoRenew: boolean;
  status: 'active' | 'offline';
  description: string;
};

type Customer = {
  customerId: string;
  email: string;
  name: string;
  country: string;
  province: string;
  city: string;
  ipRegion: string;
  planCode: CustomerPlan['code'];
  planExpiresAt: string;
  ownDevices: number;
  invitedDevices: number;
  relayUsedGb: number;
  status: 'active' | 'limited' | 'expired' | 'disabled';
};

type Renewal = {
  renewalId: string;
  customerEmail: string;
  planCode: CustomerPlan['code'];
  period: 'monthly' | 'yearly' | 'custom';
  amount: number;
  paidAt: string;
  validUntil: string;
  source: 'manual' | 'wechat' | 'alipay' | 'bank';
  operator: string;
};

type Order = {
  orderId: string;
  customerEmail: string;
  productName: string;
  productType: Product['type'];
  amount: number;
  payStatus: 'pending' | 'paid' | 'refunded' | 'closed';
  provisionStatus: 'pending' | 'provisioned' | 'failed';
  createdAt: string;
  paidAt?: string;
  validUntil?: string;
  channel: 'manual' | 'wechat' | 'alipay' | 'bank';
};

@Component({
  selector: 'ops-root',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    OverviewPageComponent,
    OperatorsPageComponent,
    RelayNodesPageComponent,
    CustomersPageComponent,
    ProductsPageComponent,
    OrdersPageComponent,
    RenewalsPageComponent,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
  encapsulation: ViewEncapsulation.None,
})
export class AppComponent {
  readonly navItems: Array<{ id: NavId; label: string; desc: string }> = [
    { id: 'overview', label: '运营管理', desc: '平台指标与待处理事项' },
    { id: 'operators', label: '运营用户', desc: '后台账号与角色' },
    { id: 'relayNodes', label: '中继节点', desc: 'Relay/DERP 容量管理' },
    { id: 'customers', label: '客户管理', desc: '客户资源与限流状态' },
    { id: 'products', label: '商品管理', desc: '客户级别、套餐商品与流量包' },
    { id: 'orders', label: '订单管理', desc: '购买、支付与开通状态' },
    { id: 'renewals', label: '续费管理', desc: '有效期和手动续费' },
  ];

  active: NavId = 'overview';
  showAssignPlanDialog = false;
  showCurrentPasswordDialog = false;
  showOperatorPasswordDialog = false;
  selectedCustomer: Customer | null = null;
  selectedOperator: OperatorUser | null = null;
  assignPlanCode: CustomerPlan['code'] = 'pro';
  assignExpiresAt = '2027-05-09';
  renewalAmount = 299;
  oldPassword = '';
  newPassword = '';
  confirmPassword = '';
  operatorNewPassword = '';
  operatorConfirmPassword = '';
  passwordMessage = '';

  get vm(): this {
    return this;
  }

  operators: OperatorUser[] = [
    { operatorId: 'op-000001', name: '平台管理员', email: 'admin@slan.com', role: 'owner', status: 'active', lastLoginAt: '2026-05-09 09:20' },
    { operatorId: 'op-000002', name: '运营值班', email: 'ops@slan.com', role: 'ops', status: 'active', lastLoginAt: '2026-05-08 22:10' },
    { operatorId: 'op-000003', name: '财务', email: 'finance@slan.com', role: 'finance', status: 'active', lastLoginAt: '2026-05-07 18:42' },
  ];

  relayNodes: RelayNode[] = [
    { nodeId: 'relay-hk-001', name: '香港 Relay 1', region: 'ap-east-1', transport: 'relay_udp', publicAddr: 'udp://hk1.relay.slan.com:3478', maxBandwidthMbps: 1000, monthlyTrafficGb: 20480, usedTrafficGb: 6830, maxSessions: 8000, activeSessions: 2310, status: 'active', health: 'healthy' },
    { nodeId: 'derp-tokyo-001', name: '东京 DERP 1', region: 'ap-northeast-1', transport: 'derp_tcp_tls_443', publicAddr: 'https://tyo1.derp.slan.com', maxBandwidthMbps: 500, monthlyTrafficGb: 10240, usedTrafficGb: 9120, maxSessions: 4000, activeSessions: 3380, status: 'maintenance', health: 'warning' },
    { nodeId: 'relay-sg-001', name: '新加坡 Relay 1', region: 'ap-southeast-1', transport: 'relay_udp', publicAddr: 'udp://sg1.relay.slan.com:3478', maxBandwidthMbps: 800, monthlyTrafficGb: 15360, usedTrafficGb: 4210, maxSessions: 6000, activeSessions: 1740, status: 'active', health: 'healthy' },
  ];

  plans: CustomerPlan[] = [
    { code: 'free', name: '免费版', ownDeviceLimit: 5, invitedDeviceLimit: 5, totalDeviceLimit: 10, relayMonthlyGb: 50, relayBandwidthMbps: 5, relayThrottleMbps: 1, p2pUnlimited: true, customDomain: false, acl: false, dedicatedRelay: false, auditLog: false, apiAccess: false, monthlyPrice: 0, yearlyPrice: 0 },
    { code: 'pro', name: '专业版', ownDeviceLimit: 30, invitedDeviceLimit: 100, totalDeviceLimit: 130, relayMonthlyGb: 1024, relayBandwidthMbps: 100, relayThrottleMbps: 20, p2pUnlimited: true, customDomain: true, acl: true, dedicatedRelay: false, auditLog: false, apiAccess: false, monthlyPrice: 39, yearlyPrice: 299 },
    { code: 'enterprise', name: '企业版', ownDeviceLimit: 500, invitedDeviceLimit: 5000, totalDeviceLimit: 5500, relayMonthlyGb: 10240, relayBandwidthMbps: 1000, relayThrottleMbps: 200, p2pUnlimited: true, customDomain: true, acl: true, dedicatedRelay: true, auditLog: true, apiAccess: true, monthlyPrice: 0, yearlyPrice: 2999 },
  ];

  products: Product[] = [
    { productId: 'prod-free', name: '免费版', type: 'plan', planCode: 'free', period: 'monthly', validDays: 30, relayTrafficGb: 50, relayBandwidthMbps: 5, listPrice: 0, salePrice: 0, currency: 'CNY', autoRenew: false, status: 'active', description: '默认开通，P2P 不限，Relay 超限降速' },
    { productId: 'prod-pro-month', name: '专业版月付', type: 'plan', planCode: 'pro', period: 'monthly', validDays: 31, relayTrafficGb: 1024, relayBandwidthMbps: 100, listPrice: 39, salePrice: 39, currency: 'CNY', autoRenew: true, status: 'active', description: '适合个人和小团队，支持自定义域名和 ACL' },
    { productId: 'prod-pro-year', name: '专业版年付', type: 'plan', planCode: 'pro', period: 'yearly', validDays: 365, relayTrafficGb: 1024, relayBandwidthMbps: 100, listPrice: 468, salePrice: 299, currency: 'CNY', autoRenew: true, status: 'active', description: '专业版年付优惠' },
    { productId: 'prod-relay-100g', name: 'Relay 加油包 100GB', type: 'traffic_pack', period: 'one_time', validDays: 31, relayTrafficGb: 100, relayBandwidthMbps: 0, listPrice: 19, salePrice: 19, currency: 'CNY', autoRenew: false, status: 'active', description: '当月 Relay 流量加购，不改变客户级别' },
    { productId: 'prod-enterprise', name: '企业定制包', type: 'enterprise', planCode: 'enterprise', period: 'contract', validDays: 365, relayTrafficGb: 10240, relayBandwidthMbps: 1000, listPrice: 29999, salePrice: 2999, currency: 'CNY', autoRenew: false, status: 'active', description: '专属 Relay、审计日志、API 接入，按合同报价' },
  ];

  customers: Customer[] = [
    { customerId: 'cust-000001', email: 'alice@vlan.com', name: 'Alice', country: '中国', province: '广东', city: '深圳', ipRegion: '华南', planCode: 'pro', planExpiresAt: '2027-05-09', ownDevices: 3, invitedDevices: 2, relayUsedGb: 318, status: 'active' },
    { customerId: 'cust-000002', email: 'bob@vlan.com', name: 'Bob', country: '中国', province: '上海', city: '上海', ipRegion: '华东', planCode: 'free', planExpiresAt: '2026-06-01', ownDevices: 5, invitedDevices: 4, relayUsedGb: 48, status: 'limited' },
    { customerId: 'cust-000003', email: 'corp@example.com', name: '企业客户', country: '新加坡', province: '-', city: 'Singapore', ipRegion: '亚太', planCode: 'enterprise', planExpiresAt: '2028-01-01', ownDevices: 560, invitedDevices: 820, relayUsedGb: 6230, status: 'active' },
  ];

  renewals: Renewal[] = [
    { renewalId: 'renew-000001', customerEmail: 'alice@vlan.com', planCode: 'pro', period: 'yearly', amount: 299, paidAt: '2026-05-09', validUntil: '2027-05-09', source: 'alipay', operator: 'admin@slan.com' },
    { renewalId: 'renew-000002', customerEmail: 'corp@example.com', planCode: 'enterprise', period: 'yearly', amount: 12999, paidAt: '2026-01-01', validUntil: '2028-01-01', source: 'bank', operator: 'finance@slan.com' },
  ];

  orders: Order[] = [
    { orderId: 'ord-202605090001', customerEmail: 'alice@vlan.com', productName: '专业版年付', productType: 'plan', amount: 299, payStatus: 'paid', provisionStatus: 'provisioned', createdAt: '2026-05-09 09:10', paidAt: '2026-05-09 09:12', validUntil: '2027-05-09', channel: 'alipay' },
    { orderId: 'ord-202605090002', customerEmail: 'bob@vlan.com', productName: 'Relay 加油包 100GB', productType: 'traffic_pack', amount: 19, payStatus: 'pending', provisionStatus: 'pending', createdAt: '2026-05-09 10:40', channel: 'wechat' },
    { orderId: 'ord-202601010001', customerEmail: 'corp@example.com', productName: '企业定制包', productType: 'enterprise', amount: 12999, payStatus: 'paid', provisionStatus: 'provisioned', createdAt: '2026-01-01 13:20', paidAt: '2026-01-01 14:02', validUntil: '2028-01-01', channel: 'bank' },
  ];

  get activeNav() {
    return this.navItems.find((item) => item.id === this.active) ?? this.navItems[0];
  }

  get totalCustomers(): number {
    return this.customers.length;
  }

  get totalRelayUsedGb(): number {
    return this.relayNodes.reduce((sum, node) => sum + node.usedTrafficGb, 0);
  }

  get limitedCustomers(): number {
    return this.customers.filter((customer) => customer.status === 'limited').length;
  }

  get activeRelayNodes(): number {
    return this.relayNodes.filter((node) => node.status === 'active').length;
  }

  get paidOrders(): Order[] {
    return this.orders.filter((order) => order.payStatus === 'paid');
  }

  get todayRevenue(): number {
    return this.paidOrders
      .filter((order) => order.paidAt?.startsWith('2026-05-09'))
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get monthlyRevenue(): number {
    return this.paidOrders
      .filter((order) => order.paidAt?.startsWith('2026-05'))
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get yearlyRevenue(): number {
    return this.paidOrders
      .filter((order) => order.paidAt?.startsWith('2026'))
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get pendingRevenue(): number {
    return this.orders
      .filter((order) => order.payStatus === 'pending')
      .reduce((sum, order) => sum + order.amount, 0);
  }

  get paidOrderCount(): number {
    return this.orders.filter((order) => order.payStatus === 'paid').length;
  }

  get pendingOrderCount(): number {
    return this.orders.filter((order) => order.payStatus === 'pending').length;
  }

  get provisionedOrderCount(): number {
    return this.orders.filter((order) => order.provisionStatus === 'provisioned').length;
  }

  get closedOrderCount(): number {
    return this.orders.filter((order) => order.payStatus === 'closed' || order.payStatus === 'refunded').length;
  }

  get addressStats(): Array<{ label: string; count: number; percent: number }> {
    const total = Math.max(1, this.customers.length);
    const counts = new Map<string, number>();
    for (const customer of this.customers) {
      const label = `${customer.country} / ${customer.city}`;
      counts.set(label, (counts.get(label) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([label, count]) => ({ label, count, percent: Math.round((count / total) * 100) }))
      .sort((a, b) => b.count - a.count);
  }

  get regionStats(): Array<{ label: string; count: number; percent: number }> {
    const total = Math.max(1, this.customers.length);
    const counts = new Map<string, number>();
    for (const customer of this.customers) {
      counts.set(customer.ipRegion, (counts.get(customer.ipRegion) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([label, count]) => ({ label, count, percent: Math.round((count / total) * 100) }))
      .sort((a, b) => b.count - a.count);
  }

  setActive(id: NavId): void {
    this.active = id;
  }

  planName(code: CustomerPlan['code']): string {
    return this.plans.find((plan) => plan.code === code)?.name ?? code;
  }

  planOf(code: CustomerPlan['code']): CustomerPlan {
    return this.plans.find((plan) => plan.code === code) ?? this.plans[0];
  }

  customerRelayPercent(customer: Customer): number {
    const plan = this.planOf(customer.planCode);
    return Math.min(100, Math.round((customer.relayUsedGb / plan.relayMonthlyGb) * 100));
  }

  relayNodePercent(node: RelayNode): number {
    return Math.min(100, Math.round((node.usedTrafficGb / node.monthlyTrafficGb) * 100));
  }

  openAssignPlan(customer: Customer): void {
    this.selectedCustomer = customer;
    this.assignPlanCode = customer.planCode;
    this.assignExpiresAt = customer.planExpiresAt;
    this.renewalAmount = this.planOf(customer.planCode).yearlyPrice;
    this.showAssignPlanDialog = true;
  }

  closeAssignPlan(): void {
    this.showAssignPlanDialog = false;
    this.selectedCustomer = null;
  }

  saveAssignPlan(): void {
    if (!this.selectedCustomer) {
      return;
    }
    this.selectedCustomer.planCode = this.assignPlanCode;
    this.selectedCustomer.planExpiresAt = this.assignExpiresAt;
    this.selectedCustomer.status = 'active';
    this.renewals = [
      {
        renewalId: `renew-${String(this.renewals.length + 1).padStart(6, '0')}`,
        customerEmail: this.selectedCustomer.email,
        planCode: this.assignPlanCode,
        period: 'custom',
        amount: this.renewalAmount,
        paidAt: '2026-05-09',
        validUntil: this.assignExpiresAt,
        source: 'manual',
        operator: 'admin@slan.com',
      },
      ...this.renewals,
    ];
    this.closeAssignPlan();
  }

  toggleOperator(operator: OperatorUser): void {
    operator.status = operator.status === 'active' ? 'disabled' : 'active';
  }

  openCurrentPasswordDialog(): void {
    this.oldPassword = '';
    this.newPassword = '';
    this.confirmPassword = '';
    this.passwordMessage = '';
    this.showCurrentPasswordDialog = true;
  }

  closeCurrentPasswordDialog(): void {
    this.showCurrentPasswordDialog = false;
  }

  saveCurrentPassword(): void {
    this.passwordMessage = this.validatePassword(this.newPassword, this.confirmPassword, true);
    if (this.passwordMessage) {
      return;
    }
    this.closeCurrentPasswordDialog();
  }

  openOperatorPasswordDialog(operator: OperatorUser): void {
    this.selectedOperator = operator;
    this.operatorNewPassword = '';
    this.operatorConfirmPassword = '';
    this.passwordMessage = '';
    this.showOperatorPasswordDialog = true;
  }

  closeOperatorPasswordDialog(): void {
    this.showOperatorPasswordDialog = false;
    this.selectedOperator = null;
  }

  saveOperatorPassword(): void {
    this.passwordMessage = this.validatePassword(this.operatorNewPassword, this.operatorConfirmPassword, false);
    if (this.passwordMessage) {
      return;
    }
    this.closeOperatorPasswordDialog();
  }

  private validatePassword(password: string, confirmPassword: string, requireOldPassword: boolean): string {
    if (requireOldPassword && !this.oldPassword.trim()) {
      return '请输入旧密码';
    }
    if (!password.trim() || !confirmPassword.trim()) {
      return '请输入新密码并确认';
    }
    if (password !== confirmPassword) {
      return '两次输入的新密码不一致';
    }
    if (password.length < 8) {
      return '新密码至少 8 位';
    }
    return '';
  }

  toggleRelayNode(node: RelayNode): void {
    node.status = node.status === 'active' ? 'disabled' : 'active';
    node.health = node.status === 'active' ? 'healthy' : 'down';
  }

  toggleProduct(product: Product): void {
    product.status = product.status === 'active' ? 'offline' : 'active';
  }
}
