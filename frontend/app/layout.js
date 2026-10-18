import "./globals.css";
import AuthProvider, { AuthGate } from "@/components/AuthProvider";
import Navbar from "@/components/Navbar";
import RealtimeProvider from "@/components/RealtimeProvider";
import UnreadMessagesProvider from "@/components/UnreadMessages";
import VerifyBanner from "@/components/VerifyBanner";

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
            <UnreadMessagesProvider>
              <Navbar />
              <main className="container">
                <VerifyBanner />
                <AuthGate>{children}</AuthGate>
              </main>
            </UnreadMessagesProvider>
          </RealtimeProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
