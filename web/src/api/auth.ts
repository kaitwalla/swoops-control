import { api } from './client';

export const authApi = {
  updatePassword: async (currentPassword: string, newPassword: string): Promise<void> => {
    return api.put('/auth/password', {
      current_password: currentPassword,
      new_password: newPassword,
    });
  },
};
