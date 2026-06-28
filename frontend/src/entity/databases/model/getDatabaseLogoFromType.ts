import { asset } from '../../../shared/basePath';

import { DatabaseType } from './DatabaseType';

export const getDatabaseLogoFromType = (type: DatabaseType) => {
  switch (type) {
    case DatabaseType.POSTGRES:
      return asset('/icons/databases/postgresql.svg');
    case DatabaseType.REDIS:
      return asset('/icons/databases/redis.svg');
    case DatabaseType.RABBITMQ:
      return asset('/icons/databases/rabbitmq.svg');
    case DatabaseType.KUBERNETES:
      return asset('/icons/databases/kubernetes.svg');
    default:
      return '';
  }
};
