import { DatabaseType } from './DatabaseType';

export const getDatabaseLogoFromType = (type: DatabaseType) => {
  switch (type) {
    case DatabaseType.POSTGRES:
      return '/icons/databases/postgresql.svg';
    case DatabaseType.MYSQL:
      return '/icons/databases/mysql.svg';
    case DatabaseType.MARIADB:
      return '/icons/databases/mariadb.svg';
    case DatabaseType.MONGODB:
      return '/icons/databases/mongodb.svg';
    case DatabaseType.REDIS:
      return '/icons/databases/redis.svg';
    case DatabaseType.RABBITMQ:
      return '/icons/databases/rabbitmq.svg';
    default:
      return '';
  }
};
