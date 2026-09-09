import dataclasses


@dataclasses.dataclass
class ContextValue:
    label: str


serialization_records: list[tuple[str, str, str]] = []
