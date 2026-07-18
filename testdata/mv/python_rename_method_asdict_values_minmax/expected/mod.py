from dataclasses import dataclass, asdict, astuple
import dataclasses


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Pair:
    # homogeneous field values — min/max over values is safe
    a: A
    c: A


@dataclass
class Box:
    # mixed field values — min/max over values must fail closed
    a: A
    b: B


def use_min_values(pair: Pair) -> int:
    x = min(asdict(pair).values(), key=lambda v: 0)
    return x.execute()


def use_max_values(pair: Pair) -> int:
    x = max(asdict(pair).values(), key=lambda v: 0)
    return x.execute()


def use_min_no_key(pair: Pair) -> int:
    x = min(asdict(pair).values())
    return x.execute()


def use_dc(pair: Pair) -> int:
    x = min(dataclasses.asdict(pair).values(), key=lambda v: 0)
    return x.execute()


def use_vars(pair: Pair) -> int:
    x = min(vars(pair).values(), key=lambda v: 0)
    return x.execute()


def use_dunder(pair: Pair) -> int:
    x = min(pair.__dict__.values(), key=lambda v: 0)
    return x.execute()


def use_assigned(pair: Pair) -> int:
    d = asdict(pair)
    x = min(d.values(), key=lambda v: 0)
    return x.execute()


def use_list(pair: Pair) -> int:
    x = min(list(asdict(pair).values()), key=lambda v: 0)
    return x.execute()


def use_walrus(pair: Pair) -> int:
    if (x := min(asdict(pair).values(), key=lambda v: 0)):
        return x.execute()
    return 0


def use_astuple(pair: Pair) -> int:
    x = min(astuple(pair), key=lambda v: 0)
    return x.execute()


def use_mixed_fail_closed(box: Box) -> int:
    # mixed A/B — leave receivers unbound (fail closed; distinct name so
    # file-global typed-locals from prior homogeneous x = min(...) do not shadow)
    item = min(asdict(box).values(), key=lambda v: 0)
    return item.run()


def use_b_still(box: Box) -> int:
    # key path still renames A; B stays put
    return asdict(box)["a"].execute() + asdict(box)["b"].run()
