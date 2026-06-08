import type { Notifier } from '../../notifiers';
import type { DatabaseType } from './DatabaseType';
import type { HealthStatus } from './HealthStatus';
import type { MariadbDatabase } from './mariadb/MariadbDatabase';
import type { MongodbDatabase } from './mongodb/MongodbDatabase';
import type { MysqlDatabase } from './mysql/MysqlDatabase';
import type { PostgresqlDatabase } from './postgresql/PostgresqlDatabase';
import type { RabbitmqDatabase } from './rabbitmq/RabbitmqDatabase';
import type { RedisDatabase } from './redis/RedisDatabase';

export interface Database {
  id: string;
  name: string;
  workspaceId: string;
  type: DatabaseType;

  postgresql?: PostgresqlDatabase;
  mysql?: MysqlDatabase;
  mariadb?: MariadbDatabase;
  mongodb?: MongodbDatabase;
  redis?: RedisDatabase;
  rabbitmq?: RabbitmqDatabase;

  notifiers: Notifier[];

  lastBackupTime?: Date;
  lastBackupErrorMessage?: string;

  healthStatus?: HealthStatus;

  isAgentTokenGenerated: boolean;
}
