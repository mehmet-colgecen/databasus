export enum KubernetesResourceType {
  SECRET = 'SECRET',
  CONFIGMAP = 'CONFIGMAP',
}

export type KubernetesNamespaceScope = 'ALL' | 'SPECIFIC';
