import packageJson from '@/package.json';

export default function Footer() {
  return (
    <footer className="bg-white border-t border-gray-100 py-4 px-8 text-center">
      <p className="text-xs text-gray-400">
        Valiant v{packageJson.version}
      </p>
    </footer>
  );
}
