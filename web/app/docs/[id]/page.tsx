import { DocumentPageClient } from "./client";

// Required for static export
export function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

export default function DocumentPage({ params }: { params: { id: string } }) {
  return <DocumentPageClient id={params.id} />;
}
