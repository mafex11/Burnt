import { InstallDetails } from "@/components/PlatformInstall";

export function Install() {
  return (
    <section id="install" className="mx-auto max-w-content px-6 py-24 text-center">
      <h2 className="text-3xl font-extrabold tracking-tight md:text-4xl">Install in one command.</h2>
      <p className="mx-auto mt-3 max-w-md text-muted">
        No Node, no dependencies, works offline. Burnt installs itself and lands straight in
        your menu bar or system tray.
      </p>
      <div className="mt-8">
        <InstallDetails />
      </div>
    </section>
  );
}
