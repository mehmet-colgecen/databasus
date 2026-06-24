import type { KubernetesNamespaceScope } from './KubernetesResourceType';

export interface KubernetesDatabase {
  id: string;
  version: string;
  resourceTypes: string[];
  namespaceScope: KubernetesNamespaceScope;
  namespaces: string[];
  objectNames: string[];
}
