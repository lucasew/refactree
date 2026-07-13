import { Header } from './components/Header';
import { Footer } from './components/Footer';

function Shell() {
  return <Header />;
}

function App() {
  return <Shell />;
}

export function render() {
  return <Footer />;
}
