import { Button } from "@/components/ui/button";

export default function Home() {
  return (
    <div className="flex flex-col min-h-screen">
      <header className="bg-background sticky top-0 z-50 w-full border-b">
        <div className="container flex h-16 items-center justify-between">
          <div className="font-bold">MiniCRM</div>
          <nav className="hidden md:flex gap-4">
            <a href="#features">Features</a>
            <a href="#solutions">Solutions</a>
            <a href="#contact">Contact</a>
          </nav>
          <Button>Login</Button>
        </div>
      </header>
      <main className="flex-1">
        <section className="py-12 md:py-24 lg:py-32">
          <div className="container text-center">
            <h1 className="text-4xl font-bold tracking-tighter sm:text-5xl md:text-6xl">Your Complete Business Solution</h1>
            <p className="mx-auto max-w-[700px] text-muted-foreground md:text-xl/relaxed lg:text-base/relaxed xl:text-xl/relaxed">
              Integrated CRM and Call Center software to streamline your sales, support, and communication.
            </p>
            <div className="mt-6">
              <Button size="lg">Get Started</Button>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}
