import type { JSX } from 'react';
import { Box, Tab, Tabs, Typography } from '@mui/material';
import PersonOutlineIcon from '@mui/icons-material/PersonOutline';
import KeyOutlinedIcon from '@mui/icons-material/KeyOutlined';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

const userTabs = [
  { label: 'Profile', icon: <PersonOutlineIcon fontSize="small" />, path: '/user/profile' },
  { label: 'Personal Access Tokens', icon: <KeyOutlinedIcon fontSize="small" />, path: '/user/tokens' },
] as const;

function resolveTabIndex(pathname: string): number {
  const match = userTabs.findIndex((t) => pathname.startsWith(t.path));
  return match >= 0 ? match : 0;
}

export default function UserLayout(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  const activeTab = resolveTabIndex(location.pathname);

  return (
    <Box>
      <Typography variant="h3" component="h1" sx={{ mb: 2 }}>
        Account Settings
      </Typography>
      <Tabs
        value={activeTab}
        onChange={(_, index: number) => navigate(userTabs[index].path)}
        sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}
      >
        {userTabs.map((tab) => (
          <Tab
            key={tab.path}
            icon={tab.icon}
            iconPosition="start"
            label={tab.label}
            sx={{ textTransform: 'none', minHeight: 48 }}
          />
        ))}
      </Tabs>
      <Outlet />
    </Box>
  );
}
