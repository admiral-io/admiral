import type { SyntheticEvent } from 'react';
import { Alert, Button, Fade, Grow, IconButton, Slide, type SlideProps } from '@mui/material';
import MuiSnackbar from '@mui/material/Snackbar';
import CloseIcon from '@mui/icons-material/Close';

import { useDispatch, useSelector } from '@/store';
import { closeSnackbar } from '@/store/slices/snackbar';

interface SnackbarActionProps {
  onClose: (event: SyntheticEvent | Event, reason?: string) => void;
  showUndo?: boolean;
  showClose?: boolean;
}

interface TransitionProps extends SlideProps {
  direction?: 'left' | 'right' | 'up' | 'down';
}

const SNACKBAR_AUTO_HIDE_DURATION = 5000;

const createTransition = ({ direction, ...props }: TransitionProps) => {
  if (!direction) return <Grow {...props} />;
  return <Slide {...props} direction={direction} />;
};

// MUI Slide `direction` is the exit direction, which is the opposite of the
// entrance direction. We name transitions by how they *appear* to the user,
// so SlideDown enters from the top (exits up → direction 'up' is wrong;
// direction 'down' means "exit downward" = "enter from top"). Corrected:
//   SlideDown  → enters sliding downward  → exits upward   → direction: 'up'  (inverted)
//   SlideUp    → enters sliding upward    → exits downward → direction: 'down'
//   SlideLeft  → enters sliding left      → exits right    → direction: 'right'
//   SlideRight → enters sliding right     → exits left     → direction: 'left'
const transitions = {
  SlideLeft: (props: SlideProps) => createTransition({ ...props, direction: 'right' }),
  SlideUp: (props: SlideProps) => createTransition({ ...props, direction: 'down' }),
  SlideRight: (props: SlideProps) => createTransition({ ...props, direction: 'left' }),
  SlideDown: (props: SlideProps) => createTransition({ ...props, direction: 'up' }),
  Grow: createTransition,
  Fade,
};

const SnackbarActions: React.FC<SnackbarActionProps> = ({
  onClose,
  showUndo = false,
  showClose = true,
}) => (
  <>
    {showUndo && (
      <Button color="secondary" size="small" onClick={onClose}>
        UNDO
      </Button>
    )}
    {showClose && (
      <IconButton size="small" aria-label="close" color="inherit" onClick={onClose}>
        <CloseIcon fontSize="small" />
      </IconButton>
    )}
  </>
);

const Snackbar = () => {
  const dispatch = useDispatch();
  const { actionButton, anchorOrigin, alert, close, message, open, transition, variant } =
    useSelector((state) => state.snackbar);

  const handleClose = (_event: SyntheticEvent | Event, reason?: string) => {
    if (reason === 'clickaway') return;
    dispatch(closeSnackbar());
  };

  const commonProps = {
    anchorOrigin,
    open,
    autoHideDuration: SNACKBAR_AUTO_HIDE_DURATION,
    onClose: handleClose,
    slots: { transition: transitions[transition] },
  };

  if (variant === 'alert') {
    return (
      <MuiSnackbar {...commonProps}>
        <Alert
          variant={alert.variant}
          color={alert.color}
          action={
            <SnackbarActions onClose={handleClose} showUndo={actionButton} showClose={close} />
          }
          sx={{
            ...(alert.variant === 'outlined' && {
              bgcolor: 'background.paper',
            }),
          }}
        >
          {message}
        </Alert>
      </MuiSnackbar>
    );
  }

  return (
    <MuiSnackbar
      {...commonProps}
      message={message}
      action={
        <SnackbarActions onClose={handleClose} showUndo={actionButton} showClose={close} />
      }
    />
  );
};

export default Snackbar;
