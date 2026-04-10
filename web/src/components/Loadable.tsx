import { Suspense, memo, type ComponentType, type ReactElement } from 'react';
import { ErrorBoundary } from 'react-error-boundary';
import { Alert, Box } from '@mui/material';

import Loader from '@/components/Loader';

interface LoadableProps {
  fallback?: NonNullable<React.ReactNode>;
}

function Loadable<P extends object>(Component: ComponentType<P>) {
  function LoadableComponent(props: P & LoadableProps): ReactElement {
    const { fallback = <Loader />, ...restProps } = props;

    const componentProps = restProps as P;

    return (
      <ErrorBoundary
        fallback={
          <Box sx={{ p: 2 }}>
            <Alert severity="error">Error loading component.</Alert>
          </Box>
        }
        onError={(error) => {
          console.error('Error loading component:', error);
        }}
      >
        <Suspense fallback={fallback}>
          <Component {...componentProps} />
        </Suspense>
      </ErrorBoundary>
    );
  }

  LoadableComponent.displayName = `Loadable(${Component.displayName || Component.name || 'Component'})`;

  return memo(LoadableComponent);
}

export default Loadable;
