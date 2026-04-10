import { RouterProvider } from 'react-router-dom';

import Snackbar from '@/components/extended/Snackbar';
import Theme from '@/theme';
import router from '@/routes';

function App() {
  return (
    <Theme>
      <RouterProvider router={router} />
      <Snackbar />
    </Theme>
  );
}

export default App;
