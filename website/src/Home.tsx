import { StrictMode } from 'react';
import App from './App';

// Astro mounts this as a client-only island, so the home keeps its own
// client-side theme and language state.
export default function Home() {
  return (
    <StrictMode>
      <App />
    </StrictMode>
  );
}
