import { useState, type FormEvent } from "react";
import { Download, FileText, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog } from "../../shared/ui/Dialog";
import { apiPost } from "../../shared/api";
import { ResourceState, useCommand, useResource } from "./api-state";
import {
  day,
  documentNames,
  today,
  type List,
  type MemberDocument,
} from "./types";

export function Documents({
  clubId,
  membershipId,
  canManage,
  beforeNavigate,
}: {
  clubId: string;
  membershipId: string;
  canManage: boolean;
  beforeNavigate: (action: () => void) => void;
}) {
  const base =
    `/api/clubs/${clubId}/participants/${membershipId}/documents` as const;
  const resource = useResource<List<MemberDocument>>(base);
  const [upload, setUpload] = useState(false),
    [removing, setRemoving] = useState<MemberDocument | null>(null);
  const command = useCommand(resource.refresh);
  return (
    <div className="documents">
      <div className="module-heading">
        <div>
          <h3>Документы участника</h3>
          <p className="help">Справки, договоры и согласия в одном месте.</p>
        </div>
        {canManage && (
          <Button
            disabled={resource.unavailable || resource.loading}
            onClick={() => setUpload(true)}
          >
            <Plus />
            Добавить документ
          </Button>
        )}
      </div>
      <ResourceState
        resource={resource}
        empty={resource.data?.items.length === 0}
        title="Документы пока не добавлены"
      >
        <p>
          {canManage
            ? "Добавьте справку, договор или согласие. Файлы доступны только семье и уполномоченным сотрудникам."
            : "Документы появятся после добавления администратором клуба."}
        </p>
      </ResourceState>
      {command.error && (
        <p className="error" role="alert">
          {command.error}
        </p>
      )}
      <ul className="document-list">
        {resource.data?.items.map((doc) => (
          <li key={doc.id}>
            <span className="document-icon">
              <FileText aria-hidden="true" />
            </span>
            <div className="document-copy">
              <strong>{doc.name}</strong>
              <p>
                {documentNames[doc.kind]} · {Math.ceil(doc.size / 1024)} КБ
              </p>
              {doc.expiresOn && (
                <span
                  className={`status-badge ${doc.expiresOn < today() ? "warning" : "neutral"}`}
                >
                  {doc.expiresOn < today() ? "Срок истёк" : "Действует до"}{" "}
                  {day(doc.expiresOn)}
                </span>
              )}
            </div>
            <div className="row-actions">
              <Button asChild variant="outline" size="icon">
                <a
                  href={`${base}/${doc.id}/download`}
                  download
                  aria-label={`Скачать ${doc.name}`}
                >
                  <Download />
                </a>
              </Button>
              {canManage && (
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Удалить ${doc.name}`}
                  disabled={command.busy}
                  onClick={() => {
                    command.clear();
                    setRemoving(doc);
                  }}
                >
                  <Trash2 />
                </Button>
              )}
            </div>
          </li>
        ))}
      </ul>
      {upload && (
        <DocumentUpload
          base={base}
          beforeNavigate={beforeNavigate}
          onCancel={() => setUpload(false)}
          onSaved={() => {
            setUpload(false);
            resource.refresh();
          }}
        />
      )}
      {removing && (
        <Dialog
          title="Удалить документ?"
          busy={command.busy}
          onCancel={() => setRemoving(null)}
        >
          <p>
            Файл «{removing.name}» больше не будет доступен семье. Это действие
            нельзя отменить.
          </p>
          {command.error && (
            <p className="error" role="alert">
              {command.error}
            </p>
          )}
          <Button
            disabled={command.busy}
            onClick={() =>
              void command.run(
                () =>
                  apiPost(`${base}/${removing.id}/delete`, {
                    version: removing.version,
                  }),
                () => setRemoving(null),
              )
            }
          >
            {command.busy ? "Удаляем…" : "Удалить документ"}
          </Button>
        </Dialog>
      )}
    </div>
  );
}
function DocumentUpload({
  base,
  onCancel,
  onSaved,
  beforeNavigate,
}: {
  base: `/api/${string}`;
  onCancel: () => void;
  onSaved: () => void;
  beforeNavigate: (action: () => void) => void;
}) {
  const [name, setName] = useState(""),
    [kind, setKind] = useState<MemberDocument["kind"]>("medical"),
    [expires, setExpires] = useState(""),
    [file, setFile] = useState<File | null>(null),
    [validation, setValidation] = useState("");
  const command = useCommand(onSaved);
  function submit(event: FormEvent) {
    event.preventDefault();
    setValidation("");
    if (!file || !file.size || file.size > 5 * 1024 * 1024) {
      setValidation("Выберите непустой PDF, JPG или PNG размером до 5 МБ.");
      return;
    }
    if (!["application/pdf", "image/jpeg", "image/png"].includes(file.type)) {
      setValidation("Поддерживаются только PDF, JPG и PNG.");
      return;
    }
    void command.run(async () => {
      const contentBase64 = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onerror = () => reject(new Error("file_read"));
        reader.onload = () => resolve(String(reader.result).split(",")[1]);
        reader.readAsDataURL(file);
      });
      return apiPost(base, {
        name: name.trim(),
        kind,
        filename: file.name,
        contentBase64,
        expiresOn: expires || null,
      });
    });
  }
  return (
    <Dialog
      title="Новый документ"
      busy={command.busy}
      onCancel={() => beforeNavigate(onCancel)}
    >
      <form onSubmit={submit} aria-busy={command.busy}>
        <label className="field">
          Название
          <input
            required
            maxLength={200}
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={command.busy}
          />
        </label>
        <label className="field">
          Тип документа
          <select
            value={kind}
            onChange={(e) => setKind(e.target.value as MemberDocument["kind"])}
            disabled={command.busy}
          >
            {Object.entries(documentNames).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          Файл
          <input
            type="file"
            required
            accept="application/pdf,image/jpeg,image/png"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            disabled={command.busy}
          />
          <span className="help">PDF, JPG или PNG · до 5 МБ</span>
        </label>
        <label className="field">
          Действует до
          <input
            type="date"
            value={expires}
            onChange={(e) => setExpires(e.target.value)}
            disabled={command.busy}
          />
          <span className="help">Оставьте пустым, если срок не ограничен.</span>
        </label>
        {(validation || command.error) && (
          <p className="error" role="alert">
            {validation || command.error}
          </p>
        )}
        <Button type="submit" disabled={command.busy}>
          {command.busy ? "Загружаем…" : "Загрузить документ"}
        </Button>
      </form>
    </Dialog>
  );
}
