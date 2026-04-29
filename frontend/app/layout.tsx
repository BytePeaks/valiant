import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import Footer from "@/components/footer";
import packageJson from "@/package.json";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Valiant",
  description: "Change Impact Radar",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${inter.className} min-h-screen flex flex-col`}>
        <div className="flex-1">
          {children}
        </div>
        <Footer />
        <div className="fixed bottom-3 right-3 z-50 pointer-events-none">
          <span className="text-[10px] text-gray-400 bg-white/80 backdrop-blur-sm border border-gray-200 rounded px-1.5 py-0.5 font-mono">
            v{packageJson.version}
          </span>
        </div>
      </body>
    </html>
  );
}
