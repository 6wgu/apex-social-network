import "./globals.css";
import AuthProvider, { AuthGate } from "@/components/AuthProvider";
import Navbar from "@/components/Navbar";

export const metadata = {
  title: "Social Network",
  description: "A Facebook-like social network",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>
        <AuthProvider>
          <Navbar />
          <main className="container">
            <AuthGate>{children}</AuthGate>
          </main>
        </AuthProvider>
      </body>
    </html>
  );
}
