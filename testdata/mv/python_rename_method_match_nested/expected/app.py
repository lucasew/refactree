class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_match_nested_list(aa: list[list[A]], bb: list[list[B]]) -> int:
    match aa:
        case [[xa, *_], *_]:
            r = xa.execute()
    match bb:
        case [[xb, *_], *_]:
            r += xb.run()
    return r


def use_match_nested_list_row(aa: list[list[A]], bb: list[list[B]]) -> int:
    match aa:
        case [row, *_]:
            r = row[0].execute()
    match bb:
        case [rowb, *_]:
            r += rowb[0].run()
    return r


def use_match_nested_tuple(aa: tuple[tuple[A, ...], ...], bb: tuple[tuple[B, ...], ...]) -> int:
    match aa:
        case ((xa, *_), *_):
            r = xa.execute()
    match bb:
        case ((xb, *_), *_):
            r += xb.run()
    return r


def use_match_nested_map(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    match da:
        case {"k": [xa, *_]}:
            r = xa.execute()
    match db:
        case {"k": [xb, *_]}:
            r += xb.run()
    return r


def use_match_nested_map_as(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    match da:
        case {"k": [xa as x, *_]}:
            r = x.execute()
    match db:
        case {"k": [xb as y, *_]}:
            r += y.run()
    return r


def use_match_nested_map_row(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    match da:
        case {"k": row}:
            r = row[0].execute()
    match db:
        case {"k": rowb}:
            r += rowb[0].run()
    return r


def use_match_nested_map_row_as(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    match da:
        case {"k": row as r}:
            x = r[0].execute()
    match db:
        case {"k": rowb as s}:
            x += s[0].run()
    return x


def use_preserves_b(bb: list[list[B]], db: dict[str, list[B]]) -> int:
    match bb:
        case [[xb, *_], *_]:
            r = xb.run()
    match db:
        case {"k": [yb, *_]}:
            r += yb.run()
    return r
