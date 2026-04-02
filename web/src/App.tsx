import { RouterProvider } from 'react-router-dom';

import NavigationScroll from '@/components/NavigationScroll';
import Snackbar from '@/components/extended/Snackbar';
import Theme from '@/theme';
import router from '@/routes';

function App() {
  return (
    <Theme>
      <NavigationScroll>
        <RouterProvider router={router} />
        <Snackbar />
      </NavigationScroll>
    </Theme>
  );
}

export default App;
