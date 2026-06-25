import { getApplicationServer } from '../../../constants';
import RequestOptions from '../../../shared/api/RequestOptions';
import { apiHelper } from '../../../shared/api/apiHelper';
import type { PostgresqlDatabase } from '../../databases';
import type { Restore } from '../model/Restore';

export const restoreApi = {
  async getRestores(backupId: string) {
    return apiHelper.fetchGetJson<Restore[]>(
      `${getApplicationServer()}/api/v1/restores/${backupId}`,
      undefined,
      true,
    );
  },

  async restoreBackup({
    backupId,
    postgresql,
  }: {
    backupId: string;
    postgresql?: PostgresqlDatabase;
  }) {
    const requestOptions: RequestOptions = new RequestOptions();
    requestOptions.setBody(
      JSON.stringify({
        postgresqlDatabase: postgresql,
      }),
    );

    return apiHelper.fetchPostJson<{ message: string }>(
      `${getApplicationServer()}/api/v1/restores/${backupId}/restore`,
      requestOptions,
    );
  },

  async cancelRestore(restoreId: string) {
    return apiHelper.fetchPostRaw(`${getApplicationServer()}/api/v1/restores/cancel/${restoreId}`);
  },
};
