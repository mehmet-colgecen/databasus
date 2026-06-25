import type { RabbitmqVersion } from './RabbitmqVersion';

export interface RabbitmqDatabase {
  id: string;
  version: RabbitmqVersion;
  host: string;
  managementPort: number;
  username: string;
  password: string;
  isHttps: boolean;
}
