import type React from 'react';
import { Outlet } from 'react-router-dom';

import ErrorContainer from '@/components/ErrorContainer';

const ErrorLayout: React.FC = () => {
  return (
    <ErrorContainer>
      <Outlet />
    </ErrorContainer>
  );
};

export default ErrorLayout;
