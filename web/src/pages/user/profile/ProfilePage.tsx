import type { JSX } from 'react';
import { Avatar, Box, Chip, Paper, Stack, Typography, useTheme } from '@mui/material';
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import { useSelector } from 'react-redux';

import type { RootState } from '@/store';
import { getValidPictureUrl, getAvatarInitial } from '@/utils/avatar';

function ProfileField({ label, value }: { label: string; value: string | undefined }) {
  return (
    <Box>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body1" sx={{ mt: 0.25 }}>
        {value || '\u2014'}
      </Typography>
    </Box>
  );
}

export default function ProfilePage(): JSX.Element {
  const theme = useTheme();
  const user = useSelector((s: RootState) => s.user);

  return (
    <Stack spacing={3}>
      <Paper
        variant="outlined"
        sx={{ p: 3, display: 'flex', alignItems: 'center', gap: 3 }}
      >
        <Avatar
          alt={user.display_name || user.email}
          src={getValidPictureUrl(user.avatar_url)}
          sx={{
            width: 72,
            height: 72,
            border: `2px solid ${theme.palette.divider}`,
            fontSize: '1.5rem',
          }}
        >
          {getAvatarInitial(user.display_name, user.email)}
        </Avatar>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="h6" noWrap>
            {user.display_name || 'User'}
          </Typography>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 0.5 }}>
            <Typography variant="body2" color="text.secondary" noWrap>
              {user.email}
            </Typography>
            <Chip
              size="small"
              icon={user.email_verified ? <CheckCircleOutlineIcon /> : <ErrorOutlineIcon />}
              label={user.email_verified ? 'Verified' : 'Unverified'}
              color={user.email_verified ? 'success' : 'warning'}
              variant="outlined"
            />
          </Stack>
        </Box>
      </Paper>

      <Paper variant="outlined" sx={{ p: 3 }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
          Account Details
        </Typography>
        <Stack spacing={2}>
          <ProfileField label="Display Name" value={user.display_name} />
          <ProfileField label="Given Name" value={user.given_name} />
          <ProfileField label="Family Name" value={user.family_name} />
          <ProfileField label="Email" value={user.email} />
          <ProfileField label="User ID" value={user.id} />
        </Stack>
      </Paper>

      <Paper variant="outlined" sx={{ p: 3 }}>
        <Typography variant="body2" color="text.secondary">
          Profile information is managed by your identity provider. To update your name, email, or
          avatar, make changes in your IdP and sign in again.
        </Typography>
      </Paper>
    </Stack>
  );
}
