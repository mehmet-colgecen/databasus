import { Button, Input, InputNumber, Switch } from 'antd';
import { useEffect, useState } from 'react';

import { IS_CLOUD } from '../../../../constants';
import { type Database, databaseApi } from '../../../../entity/databases';
import { ToastHelper } from '../../../../shared/toast';

interface Props {
  database: Database;

  isShowCancelButton?: boolean;
  onCancel: () => void;

  isShowBackButton: boolean;
  onBack: () => void;

  saveButtonText?: string;
  isSaveToApi: boolean;
  onSaved: (database: Database) => void;
}

export const EditRabbitmqSpecificDataComponent = ({
  database,

  isShowCancelButton,
  onCancel,

  isShowBackButton,
  onBack,

  saveButtonText,
  isSaveToApi,
  onSaved,
}: Props) => {
  const [editingDatabase, setEditingDatabase] = useState<Database>();
  const [isSaving, setIsSaving] = useState(false);

  const [isConnectionTested, setIsConnectionTested] = useState(false);
  const [isTestingConnection, setIsTestingConnection] = useState(false);
  const [isConnectionFailed, setIsConnectionFailed] = useState(false);

  const testConnection = async () => {
    if (!editingDatabase?.rabbitmq) return;

    setIsTestingConnection(true);
    setIsConnectionFailed(false);

    const trimmedDatabase = {
      ...editingDatabase,
      rabbitmq: {
        ...editingDatabase.rabbitmq,
        password: editingDatabase.rabbitmq.password?.trim(),
      },
    };

    try {
      await databaseApi.testDatabaseConnectionDirect(trimmedDatabase);
      setIsConnectionTested(true);
      ToastHelper.showToast({
        title: 'Connection test passed',
        description: 'You can continue with the next step',
      });
    } catch (e) {
      setIsConnectionFailed(true);
      alert((e as Error).message);
    }

    setIsTestingConnection(false);
  };

  const saveDatabase = async () => {
    if (!editingDatabase?.rabbitmq) return;

    const trimmedDatabase = {
      ...editingDatabase,
      rabbitmq: {
        ...editingDatabase.rabbitmq,
        password: editingDatabase.rabbitmq.password?.trim(),
      },
    };

    if (isSaveToApi) {
      setIsSaving(true);

      try {
        await databaseApi.updateDatabase(trimmedDatabase);
      } catch (e) {
        alert((e as Error).message);
      }

      setIsSaving(false);
    }

    onSaved(trimmedDatabase);
  };

  useEffect(() => {
    setIsSaving(false);
    setIsConnectionTested(false);
    setIsTestingConnection(false);
    setIsConnectionFailed(false);

    setEditingDatabase({ ...database });
  }, [database]);

  if (!editingDatabase) return null;

  let isAllFieldsFilled = true;
  if (!editingDatabase.rabbitmq?.host) isAllFieldsFilled = false;
  if (!editingDatabase.rabbitmq?.managementPort) isAllFieldsFilled = false;
  if (!editingDatabase.rabbitmq?.username) isAllFieldsFilled = false;
  if (!editingDatabase.id && !editingDatabase.rabbitmq?.password) isAllFieldsFilled = false;

  const isLocalhostDb =
    editingDatabase.rabbitmq?.host?.includes('localhost') ||
    editingDatabase.rabbitmq?.host?.includes('127.0.0.1');

  return (
    <div>
      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Host</div>
        <Input
          value={editingDatabase.rabbitmq?.host}
          onChange={(e) => {
            if (!editingDatabase.rabbitmq) return;

            setEditingDatabase({
              ...editingDatabase,
              rabbitmq: {
                ...editingDatabase.rabbitmq,
                host: e.target.value.trim().replace('https://', '').replace('http://', ''),
              },
            });
            setIsConnectionTested(false);
          }}
          size="small"
          className="max-w-[200px] grow"
          placeholder="Enter RabbitMQ host"
        />
      </div>

      {isLocalhostDb && !IS_CLOUD && (
        <div className="mb-1 flex">
          <div className="min-w-[150px]" />
          <div className="max-w-[200px] text-xs text-gray-500 dark:text-gray-400">
            Please{' '}
            <a
              href="https://databasus.com/faq/localhost"
              target="_blank"
              rel="noreferrer"
              className="!text-blue-600 dark:!text-blue-400"
            >
              read this document
            </a>{' '}
            to study how to backup local database
          </div>
        </div>
      )}

      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Management port</div>
        <InputNumber
          type="number"
          value={editingDatabase.rabbitmq?.managementPort}
          onChange={(value) => {
            if (!editingDatabase.rabbitmq || value === null) return;

            setEditingDatabase({
              ...editingDatabase,
              rabbitmq: { ...editingDatabase.rabbitmq, managementPort: value },
            });
            setIsConnectionTested(false);
          }}
          size="small"
          className="max-w-[200px] grow"
          placeholder="15672"
        />
      </div>

      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Username</div>
        <Input
          value={editingDatabase.rabbitmq?.username}
          onChange={(e) => {
            if (!editingDatabase.rabbitmq) return;

            setEditingDatabase({
              ...editingDatabase,
              rabbitmq: { ...editingDatabase.rabbitmq, username: e.target.value.trim() },
            });
            setIsConnectionTested(false);
          }}
          size="small"
          className="max-w-[200px] grow"
          placeholder="Enter RabbitMQ username"
        />
      </div>

      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Password</div>
        <Input.Password
          value={editingDatabase.rabbitmq?.password}
          onChange={(e) => {
            if (!editingDatabase.rabbitmq) return;

            setEditingDatabase({
              ...editingDatabase,
              rabbitmq: { ...editingDatabase.rabbitmq, password: e.target.value },
            });
            setIsConnectionTested(false);
          }}
          size="small"
          className="max-w-[200px] grow"
          placeholder="Enter RabbitMQ password"
          autoComplete="off"
          data-1p-ignore
          data-lpignore="true"
          data-form-type="other"
        />
      </div>

      <div className="mb-5 flex w-full items-center">
        <div className="min-w-[150px]">Use HTTPS</div>
        <Switch
          checked={editingDatabase.rabbitmq?.isHttps}
          onChange={(checked) => {
            if (!editingDatabase.rabbitmq) return;

            setEditingDatabase({
              ...editingDatabase,
              rabbitmq: { ...editingDatabase.rabbitmq, isHttps: checked },
            });
            setIsConnectionTested(false);
          }}
          size="small"
        />
      </div>

      <div className="mt-5 flex">
        {isShowCancelButton && (
          <Button className="mr-1" danger ghost onClick={() => onCancel()}>
            Cancel
          </Button>
        )}

        {isShowBackButton && (
          <Button className="mr-auto" type="primary" ghost onClick={() => onBack()}>
            Back
          </Button>
        )}

        {!isConnectionTested && (
          <Button
            type="primary"
            onClick={() => testConnection()}
            loading={isTestingConnection}
            disabled={!isAllFieldsFilled}
            className="mr-5"
          >
            Test connection
          </Button>
        )}

        {isConnectionTested && (
          <Button
            type="primary"
            onClick={() => saveDatabase()}
            loading={isSaving}
            disabled={!isAllFieldsFilled}
            className="mr-5"
          >
            {saveButtonText || 'Save'}
          </Button>
        )}
      </div>

      {isConnectionFailed && !IS_CLOUD && (
        <div className="mt-3 text-sm text-gray-500 dark:text-gray-400">
          If your database uses IP whitelist, make sure Databasus server IP is added to the allowed
          list.
        </div>
      )}
    </div>
  );
};
