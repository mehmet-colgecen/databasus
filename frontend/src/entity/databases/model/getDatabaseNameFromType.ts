import { DatabaseType } from './DatabaseType';

export const getDatabaseNameFromType = (type: DatabaseType) => {
  switch (type) {
    case DatabaseType.POSTGRES:
      return 'PostgreSQL';
    case DatabaseType.REDIS:
      return 'Redis';
    case DatabaseType.RABBITMQ:
      return 'RabbitMQ';
    case DatabaseType.KUBERNETES:
      return 'Kubernetes';
    default:
      return '';
  }
};
