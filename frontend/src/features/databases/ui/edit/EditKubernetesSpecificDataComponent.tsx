import { Alert, Button, Checkbox, Radio, Select } from 'antd';
import { useEffect, useState } from 'react';

import { type Database, KubernetesResourceType, databaseApi } from '../../../../entity/databases';
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

export const EditKubernetesSpecificDataComponent = ({
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

  const updateKubernetes = (changes: Partial<NonNullable<Database['kubernetes']>>) => {
    if (!editingDatabase?.kubernetes) return;
    setEditingDatabase({
      ...editingDatabase,
      kubernetes: { ...editingDatabase.kubernetes, ...changes },
    });
    setIsConnectionTested(false);
  };

  const testConnection = async () => {
    if (!editingDatabase?.kubernetes) return;

    setIsTestingConnection(true);
    try {
      await databaseApi.testDatabaseConnectionDirect(editingDatabase);
      setIsConnectionTested(true);
      ToastHelper.showToast({
        title: 'Connection test passed',
        description: 'You can continue with the next step',
      });
    } catch (e) {
      alert((e as Error).message);
    }
    setIsTestingConnection(false);
  };

  const saveDatabase = async () => {
    if (!editingDatabase?.kubernetes) return;

    if (isSaveToApi) {
      setIsSaving(true);
      try {
        await databaseApi.updateDatabase(editingDatabase);
      } catch (e) {
        alert((e as Error).message);
      }
      setIsSaving(false);
    }

    onSaved(editingDatabase);
  };

  useEffect(() => {
    setIsSaving(false);
    setIsConnectionTested(false);
    setIsTestingConnection(false);
    setEditingDatabase({ ...database });
  }, [database]);

  if (!editingDatabase?.kubernetes) return null;

  const kubernetes = editingDatabase.kubernetes;
  const isSecretSelected = kubernetes.resourceTypes.includes(KubernetesResourceType.SECRET);
  const isSpecificScope = kubernetes.namespaceScope === 'SPECIFIC';

  const isAllFieldsFilled =
    kubernetes.resourceTypes.length > 0 && (!isSpecificScope || kubernetes.namespaces.length > 0);

  return (
    <div>
      <div className="mb-3 flex w-full items-start">
        <div className="min-w-[150px] pt-1">Resource types</div>
        <Checkbox.Group
          value={kubernetes.resourceTypes}
          onChange={(values) => updateKubernetes({ resourceTypes: values as string[] })}
          options={[
            { label: 'Secrets', value: KubernetesResourceType.SECRET },
            { label: 'ConfigMaps', value: KubernetesResourceType.CONFIGMAP },
          ]}
        />
      </div>

      {isSecretSelected && (
        <Alert
          className="mb-3"
          type="warning"
          showIcon
          message="Secrets contain sensitive data. Enable backup encryption in the next step so the exported values are not stored in plain base64."
        />
      )}

      <div className="mb-3 flex w-full items-start">
        <div className="min-w-[150px] pt-1">Namespaces</div>
        <Radio.Group
          value={kubernetes.namespaceScope}
          onChange={(e) => updateKubernetes({ namespaceScope: e.target.value })}
        >
          <Radio value="ALL">All namespaces</Radio>
          <Radio value="SPECIFIC">Specific namespaces</Radio>
        </Radio.Group>
      </div>

      {isSpecificScope && (
        <div className="mb-3 flex w-full items-center">
          <div className="min-w-[150px]">Namespace list</div>
          <Select
            mode="tags"
            value={kubernetes.namespaces}
            onChange={(values) => updateKubernetes({ namespaces: values })}
            size="small"
            className="max-w-[280px] grow"
            placeholder="Type a namespace and press Enter"
            tokenSeparators={[',', ' ']}
          />
        </div>
      )}

      <div className="mb-5 flex w-full items-center">
        <div className="min-w-[150px]">Object names</div>
        <Select
          mode="tags"
          value={kubernetes.objectNames}
          onChange={(values) => updateKubernetes({ objectNames: values })}
          size="small"
          className="max-w-[280px] grow"
          placeholder="Optional - leave empty for all objects"
          tokenSeparators={[',', ' ']}
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
    </div>
  );
};
