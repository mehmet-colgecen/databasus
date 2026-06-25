import { DatabaseType } from './DatabaseType';

export const getDatabaseLogoFromType = (type: DatabaseType) => {
  switch (type) {
    case DatabaseType.POSTGRES:
      return '/icons/databases/postgresql.svg';
    case DatabaseType.REDIS:
      return '/icons/databases/redis.svg';
    case DatabaseType.RABBITMQ:
      return '/icons/databases/rabbitmq.svg';
    case DatabaseType.KUBERNETES:
      return '/icons/databases/kubernetes.svg';
    default:
      return '';
  }
};
