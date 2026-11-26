import { useNavigate, useRouter } from "@tanstack/react-router";
import { KeyValueMetadataCard } from "@/features/keyvalue/components/key-value-metadata-card";
import { KeyValueMaxmemoryPolicySection } from "@/features/keyvalue/components/key-value-maxmemory-policy-section";
import { KeyValuePersistenceModeSection } from "@/features/keyvalue/components/key-value-persistence-mode-section";
import { KeyValuePlanSection } from "@/features/keyvalue/components/key-value-plan-section";
import { ConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
import { KeyValueDangerActions } from "@/features/keyvalue/components/key-value-danger-actions";
import { useKeyValueDetail } from "@/features/keyvalue/lib/key-value-detail";

/**
 * A Key Value instance's Info page, at its bare URL and at `/info`, as on
 * Render (w3/m649, docs/render-artifacts/key-value.md § Pages). Render's Info
 * holds General (with its Maxmemory Policy and Persistence Mode fields),
 * Compute, Connections, Networking and the delete card, in that order; bex's
 * Details, Maxmemory policy and Persistence mode cards, then its plan, its
 * Connections, its Networking and its danger zone follow it. Each card keeps
 * the id the old Overview gave it, so its anchor still lands on it.
 */
export function KeyValueInfoPage() {
  const { keyValue, refetch } = useKeyValueDetail();
  const navigate = useNavigate();
  const router = useRouter();

  return (
    <>
      <section id="metadata" className="scroll-mt-6">
        <KeyValueMetadataCard
          keyValue={keyValue}
          onRenamed={() => void router.invalidate()}
        />
      </section>
      <section id="maxmemory-policy" className="scroll-mt-6">
        <KeyValueMaxmemoryPolicySection
          id={keyValue.id}
          onChanged={() => void refetch()}
        />
      </section>
      <section id="persistence-mode" className="scroll-mt-6">
        {/* Keyed by the instance, so an armed confirmation never carries over
            to another one. */}
        <KeyValuePersistenceModeSection
          key={keyValue.id}
          id={keyValue.id}
          name={keyValue.name}
          onChanged={() => void refetch()}
        />
      </section>
      <section id="plan" className="scroll-mt-6">
        <KeyValuePlanSection
          keyValue={keyValue}
          onChanged={() => void refetch()}
        />
      </section>
      <section id="connection" className="scroll-mt-6">
        {/* A revealed snapshot belongs to one publication and internal-auth
            state: when a confirmed change flips `public` or `internalAuth`,
            remount so the old URL is no longer copyable and a fresh Reveal is
            required (w4/222). Renames and status polls keep it. */}
        <ConnectionInfoPanel
          key={`${keyValue.id}:${keyValue.public}:${keyValue.internalAuth}`}
          id={keyValue.id}
          name={keyValue.name}
          internalAuth={keyValue.internalAuth ?? false}
          isPublic={keyValue.public}
        />
      </section>
      <section id="networking" className="scroll-mt-6">
        <KeyValueNetworkingPanel id={keyValue.id} />
      </section>
      <section id="danger-zone" className="scroll-mt-6">
        <KeyValueDangerActions
          keyValue={keyValue}
          onDeleted={() => void navigate({ to: "/", replace: true })}
          onChanged={() => void refetch()}
        />
      </section>
    </>
  );
}
