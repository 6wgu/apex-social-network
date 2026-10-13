import "./globals.css";
import AuthProvider, { AuthGate } from "@/components/AuthProvider";
import Navbar from "@/components/Navbar";
import RealtimeProvider from "@/components/RealtimeProvider";

export const metadata = {
  title: "Social Network",
  description: "A Facebook-like social network",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>
        <AuthProvider>
          <RealtimeProvider>
            <Navbar />
            <main className="container">
              <AuthGate>{children}</AuthGate>
            </main>
          </RealtimeProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
