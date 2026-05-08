import { WorkspacePanel } from './app.models';

export type WorkspaceRoutePanel = {
  panel: WorkspacePanel;
  selectedZoneId?: string;
  selectedSecurityGroupId?: string;
};

export function workspacePanelPath(workspaceId: string, panel: WorkspacePanel, selectedZoneId: string, selectedSecurityGroupId: string): string {
  const base = `/space/${encodeURIComponent(workspaceId)}`;
  switch (panel) {
    case 'zones':
      return `${base}/zones`;
    case 'records':
      return `${base}/zones/${encodeURIComponent(selectedZoneId)}/records`;
    case 'publicMappings':
      return `${base}/public-mappings`;
    case 'securityGroups':
      return `${base}/security-groups`;
    case 'securityRules':
      return `${base}/security-groups/${encodeURIComponent(selectedSecurityGroupId)}/rules`;
    default:
      return base;
  }
}

export function panelFromRoute(route: string): WorkspaceRoutePanel {
  if (route.startsWith('zones/') && route.endsWith('/records')) {
    const parts = route.split('/');
    return { panel: 'records', selectedZoneId: decodeURIComponent(parts[1] || 'default') };
  }
  if (route.startsWith('security-groups/') && route.endsWith('/rules')) {
    const parts = route.split('/');
    return { panel: 'securityRules', selectedSecurityGroupId: decodeURIComponent(parts[1] || 'default') };
  }
  switch (route) {
    case '':
    case 'devices':
    case 'members':
      return { panel: 'zones' };
    case 'zones':
      return { panel: 'zones' };
    case 'public-mappings':
      return { panel: 'publicMappings' };
    case 'security-groups':
      return { panel: 'securityGroups' };
    default:
      return { panel: 'zones' };
  }
}
