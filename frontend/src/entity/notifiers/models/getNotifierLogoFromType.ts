import { asset } from '../../../shared/basePath';

import { NotifierType } from './NotifierType';

export const getNotifierLogoFromType = (type: NotifierType) => {
  switch (type) {
    case NotifierType.EMAIL:
      return asset('/icons/notifiers/email.svg');
    case NotifierType.TELEGRAM:
      return asset('/icons/notifiers/telegram.svg');
    case NotifierType.WEBHOOK:
      return asset('/icons/notifiers/webhook.svg');
    case NotifierType.SLACK:
      return asset('/icons/notifiers/slack.svg');
    case NotifierType.DISCORD:
      return asset('/icons/notifiers/discord.svg');
    case NotifierType.TEAMS:
      return asset('/icons/notifiers/teams.svg');
    default:
      return '';
  }
};
