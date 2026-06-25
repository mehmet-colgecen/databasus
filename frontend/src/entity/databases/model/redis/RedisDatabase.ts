import type { RedisVersion } from './RedisVersion';

export interface RedisDatabase {
  id: string;
  version: RedisVersion;
  host: string;
  port: number;
  username: string;
  password: string;
  isTls: boolean;
}
