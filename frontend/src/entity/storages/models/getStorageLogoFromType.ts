import { asset } from '../../../shared/basePath';

import { StorageType } from './StorageType';

export const getStorageLogoFromType = (type: StorageType) => {
  switch (type) {
    case StorageType.LOCAL:
      return asset('/icons/storages/local.svg');
    case StorageType.S3:
      return asset('/icons/storages/s3.svg');
    case StorageType.GOOGLE_DRIVE:
      return asset('/icons/storages/google-drive.svg');
    case StorageType.NAS:
      return asset('/icons/storages/nas.svg');
    case StorageType.AZURE_BLOB:
      return asset('/icons/storages/azure.svg');
    case StorageType.FTP:
      return asset('/icons/storages/ftp.svg');
    case StorageType.SFTP:
      return asset('/icons/storages/sftp.svg');
    case StorageType.RCLONE:
      return asset('/icons/storages/rclone.svg');
    default:
      return '';
  }
};
