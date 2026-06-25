import { Modal } from 'antd';
import { useState } from 'react';

import {
  type Database,
  DatabaseType,
  PostgresBackupType,
  databaseApi,
} from '../../../../entity/databases';
import { CreateReadOnlyComponent } from './CreateReadOnlyComponent';
import { EditKubernetesSpecificDataComponent } from './EditKubernetesSpecificDataComponent';
import { EditPostgreSqlSpecificDataComponent } from './EditPostgreSqlSpecificDataComponent';
import { EditRabbitmqSpecificDataComponent } from './EditRabbitmqSpecificDataComponent';
import { EditRedisSpecificDataComponent } from './EditRedisSpecificDataComponent';

interface Props {
  database: Database;

  isShowCancelButton?: boolean;
  onCancel: () => void;

  isShowBackButton: boolean;
  onBack: () => void;

  saveButtonText?: string;
  isSaveToApi: boolean;
  onSaved: (database: Database) => void;

  isShowDbName?: boolean;
  isRestoreMode?: boolean;
}

export const EditDatabaseSpecificDataComponent = ({
  database,

  isShowCancelButton,
  onCancel,

  isShowBackButton,
  onBack,

  saveButtonText,
  isSaveToApi,
  onSaved,
  isShowDbName = true,
  isRestoreMode = false,
}: Props) => {
  const [isShowReadOnlyDialog, setIsShowReadOnlyDialog] = useState(false);
  const [editingDatabase, setEditingDatabase] = useState<Database>(database);

  const saveDb = async (databaseToSave: Database) => {
    setEditingDatabase(databaseToSave);

    if (!isSaveToApi) {
      onSaved(databaseToSave);
      return;
    }

    const isWalBackup =
      databaseToSave.type === DatabaseType.POSTGRES &&
      databaseToSave.postgresql?.backupType === PostgresBackupType.WAL_V1;

    const isReadOnlyUserNotSupported =
      databaseToSave.type === DatabaseType.REDIS ||
      databaseToSave.type === DatabaseType.RABBITMQ ||
      databaseToSave.type === DatabaseType.KUBERNETES;

    if (isWalBackup || isReadOnlyUserNotSupported) {
      onSaved(databaseToSave);
      return;
    }

    try {
      const result = await databaseApi.isUserReadOnly(databaseToSave);

      if (result.isReadOnly) {
        onSaved(databaseToSave);
      } else {
        setIsShowReadOnlyDialog(true);
      }
    } catch (e) {
      alert((e as Error).message);
    }
  };

  const onReadOnlyUserCreated = (updatedDatabase: Database) => {
    setEditingDatabase(updatedDatabase);
    setIsShowReadOnlyDialog(false);
  };

  const skipReadOnlyUser = () => {
    setIsShowReadOnlyDialog(false);
    onSaved(editingDatabase);
  };

  if (isShowReadOnlyDialog) {
    return (
      <Modal
        title="Create read-only user"
        footer={<div />}
        open={isShowReadOnlyDialog}
        onCancel={() => setIsShowReadOnlyDialog(false)}
        maskClosable={false}
        width={450}
      >
        <CreateReadOnlyComponent
          database={editingDatabase}
          onReadOnlyUserUpdated={(db) => {
            console.log('onReadOnlyUserUpdated', db);
            onReadOnlyUserCreated(db);
          }}
          onGoBack={() => {
            setIsShowReadOnlyDialog(false);
          }}
          onSkipped={() => {
            skipReadOnlyUser();
          }}
          onAlreadyExists={() => {
            console.log('onAlreadyExists');
            onSaved(editingDatabase);
          }}
        />
      </Modal>
    );
  }

  const commonProps = {
    database: editingDatabase,
    isShowCancelButton,
    onCancel,
    isShowBackButton,
    onBack,
    saveButtonText,
    isSaveToApi,
    onSaved: saveDb,
    isShowDbName,
  };

  switch (editingDatabase.type) {
    case DatabaseType.POSTGRES:
      return <EditPostgreSqlSpecificDataComponent {...commonProps} isRestoreMode={isRestoreMode} />;
    case DatabaseType.REDIS:
      return <EditRedisSpecificDataComponent {...commonProps} />;
    case DatabaseType.RABBITMQ:
      return <EditRabbitmqSpecificDataComponent {...commonProps} />;
    case DatabaseType.KUBERNETES:
      return <EditKubernetesSpecificDataComponent {...commonProps} />;
    default:
      return null;
  }
};
