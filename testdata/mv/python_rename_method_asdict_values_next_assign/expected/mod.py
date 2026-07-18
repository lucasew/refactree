from dataclasses import dataclass, asdict
import dataclasses


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A
    b: B


def use_assigned_next_iter(box: Box) -> int:
    d = asdict(box)
    return next(iter(d.values())).execute()


def use_assigned_next(box: Box) -> int:
    d = asdict(box)
    return next(d.values()).execute()


def use_dc_assigned(box: Box) -> int:
    d = dataclasses.asdict(box)
    return next(iter(d.values())).execute()


def use_vars_assigned(box: Box) -> int:
    d = vars(box)
    return next(iter(d.values())).execute()


def use_dunder_assigned(box: Box) -> int:
    d = box.__dict__
    return next(iter(d.values())).execute()


def use_walrus_assigned(box: Box) -> int:
    if (d := asdict(box)):
        return next(iter(d.values())).execute()
    return 0


def use_assign_elem(box: Box) -> int:
    d = asdict(box)
    xa = next(iter(d.values()))
    return xa.execute()


def use_direct_still(box: Box) -> int:
    # already covered by asdict_values_next; keep path warm
    return next(iter(asdict(box).values())).execute()


def use_index_still(box: Box) -> int:
    # keep B leaf untouched
    return asdict(box)["b"].run()
