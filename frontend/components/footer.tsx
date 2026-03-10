import packageJson from '@/package.json';
import { Github } from 'lucide-react';

export default function Footer() {
  return (
    <footer className="bg-white border-t border-gray-100 py-4 px-8">
      <div className="flex justify-between items-center text-xs text-gray-400">
        <p>
          Valiant v{packageJson.version}
        </p>
        <div className="flex items-center">
          <span className="mr-1">Source code available here:</span>
          <a
            href="https://github.com/BytePeaks/valiant"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center text-gray-500 hover:text-gray-700 transition-colors duration-200"
          >
            <Github className="w-3 h-3 mr-1" />
            <span>BytePeaks/valiant</span>
          </a>
        </div>
      </div>
    </footer>
  );
}
