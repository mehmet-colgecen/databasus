import { type Database, DatabaseType } from '../../../../entity/databases';
import { ShowKubernetesSpecificDataComponent } from './ShowKubernetesSpecificDataComponent';
import { ShowMariaDbSpecificDataComponent } from './ShowMariaDbSpecificDataComponent';
import { ShowMongoDbSpecificDataComponent } from './ShowMongoDbSpecificDataComponent';
import { ShowMySqlSpecificDataComponent } from './ShowMySqlSpecificDataComponent';
import { ShowPostgreSqlSpecificDataComponent } from './ShowPostgreSqlSpecificDataComponent';
import { ShowRabbitmqSpecificDataComponent } from './ShowRabbitmqSpecificDataComponent';
import { ShowRedisSpecificDataComponent } from './ShowRedisSpecificDataComponent';

interface Props {
  database: Database;
}

export const ShowDatabaseSpecificDataComponent = ({ database }: Props) => {
  switch (database.type) {
    case DatabaseType.POSTGRES:
      return <ShowPostgreSqlSpecificDataComponent database={database} />;
    case DatabaseType.MYSQL:
      return <ShowMySqlSpecificDataComponent database={database} />;
    case DatabaseType.MARIADB:
      return <ShowMariaDbSpecificDataComponent database={database} />;
    case DatabaseType.MONGODB:
      return <ShowMongoDbSpecificDataComponent database={database} />;
    case DatabaseType.REDIS:
      return <ShowRedisSpecificDataComponent database={database} />;
    case DatabaseType.RABBITMQ:
      return <ShowRabbitmqSpecificDataComponent database={database} />;
    case DatabaseType.KUBERNETES:
      return <ShowKubernetesSpecificDataComponent database={database} />;
    default:
      return null;
  }
};
