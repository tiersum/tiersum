import { Suspense } from "react";
import { DocumentPageClient } from "./client";

// Required for static export
export function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

function DocumentPageContent({ id }: { id: string }) {
  return <DocumentPageClient id={id} />;
}

export default function DocumentPage({ params }: { params: { id: string } }) {
  return (
    <Suspense fallback={<div className="min-h-screen bg-slate-950 flex items-center justify-center">
      <div className="text-slate-400">Loading...</div>
    </div>}>
      <DocumentPageContent id={params.id} />
    </Suspense>
  );
}
