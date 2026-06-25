import { type Database, DatabaseType } from '../../../../entity/databases';
import { ShowKubernetesSpecificDataComponent } from './ShowKubernetesSpecificDataComponent';
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
