type Props = {
  message: string;
};

export function ErrorBanner({ message }: Props) {
  return (
    <div className="err" role="alert">
      {message}
    </div>
  );
}
