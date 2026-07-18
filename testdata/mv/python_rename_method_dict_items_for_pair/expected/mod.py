from dataclasses import dataclass, asdict
import dataclasses


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Pair:
    a: A
    c: A


@dataclass
class Box:
    a: A
    b: B


def use_items_pair(d: dict[str, A]) -> int:
    total = 0
    for item in d.items():
        total += item[1].execute()
    return total


def use_items_assign(d: dict[str, A]) -> int:
    total = 0
    for it in d.items():
        a = it[1]
        total += a.execute()
    return total


def use_items_unpack(d: dict[str, A]) -> int:
    total = 0
    for pair in d.items():
        k, a = pair
        total += a.execute()
    return total


def use_list_items(d: dict[str, A]) -> int:
    total = 0
    for entry in list(d.items()):
        total += entry[1].execute()
    return total


def use_asdict_items(p: Pair) -> int:
    total = 0
    for kv in asdict(p).items():
        total += kv[1].execute()
    return total


def use_dc_items(p: Pair) -> int:
    total = 0
    for row in dataclasses.asdict(p).items():
        total += row[1].execute()
    return total


def use_vars_items(p: Pair) -> int:
    total = 0
    for rec in vars(p).items():
        total += rec[1].execute()
    return total


def use_dunder_items(p: Pair) -> int:
    total = 0
    for cell in p.__dict__.items():
        total += cell[1].execute()
    return total


def use_assigned(p: Pair) -> int:
    d = asdict(p)
    total = 0
    for slot in d.items():
        total += slot[1].execute()
    return total


def use_comp(d: dict[str, A]) -> int:
    return sum(el[1].execute() for el in d.items())


def use_mixed_fail_closed(box: Box) -> int:
    # mixed A/B — leave receiver unbound (fail closed; distinct name so
    # file-global pairSlots from prior homogeneous loops do not shadow)
    total = 0
    for mixed in asdict(box).items():
        total += mixed[1].run()
    return total


def use_key_fail_closed(d: dict[str, A]) -> int:
    # key slot [0] stays untyped — must not rename (distinct local)
    total = 0
    for keypair in d.items():
        total += keypair[0].run()  # type: ignore[attr-defined]
    return total


def use_b_items(d: dict[str, B]) -> int:
    # distinct name so B pairSlots does not shadow A locals above
    total = 0
    for bitem in d.items():
        total += bitem[1].run()
    return total


def use_unpack_still(d: dict[str, A]) -> int:
    # regression: for k, a in d.items() still works
    total = 0
    for k, a in d.items():
        total += a.execute()
    return total
