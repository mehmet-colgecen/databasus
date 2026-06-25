import { type Database } from '../../../../entity/databases';

interface Props {
  database: Database;
}

export const ShowKubernetesSpecificDataComponent = ({ database }: Props) => {
  const kubernetes = database.kubernetes;

  return (
    <div>
      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Resource types</div>
        <div>{kubernetes?.resourceTypes?.join(', ') || ''}</div>
      </div>

      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Namespace scope</div>
        <div>
          {kubernetes?.namespaceScope === 'SPECIFIC' ? 'Specific namespaces' : 'All namespaces'}
        </div>
      </div>

      {kubernetes?.namespaceScope === 'SPECIFIC' && (
        <div className="mb-1 flex w-full items-center">
          <div className="min-w-[150px]">Namespaces</div>
          <div>{kubernetes?.namespaces?.join(', ') || ''}</div>
        </div>
      )}

      <div className="mb-1 flex w-full items-center">
        <div className="min-w-[150px]">Object names</div>
        <div>
          {kubernetes?.objectNames?.length ? kubernetes.objectNames.join(', ') : 'All objects'}
        </div>
      </div>
    </div>
  );
};
