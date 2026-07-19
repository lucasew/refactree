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


def use_next_iter_values(box: Box) -> int:
    return next(iter(asdict(box).values())).execute()


def use_next_values(box: Box) -> int:
    return next(asdict(box).values()).execute()


def use_dc_next_iter(box: Box) -> int:
    return next(iter(dataclasses.asdict(box).values())).execute()


def use_vars_values(box: Box) -> int:
    return next(iter(vars(box).values())).execute()


def use_dunder_values(box: Box) -> int:
    return next(iter(box.__dict__.values())).execute()


def use_assign(box: Box) -> int:
    xa = next(iter(asdict(box).values()))
    return xa.execute()


def use_index_still(box: Box) -> int:
    # already covered by dataclass_asdict; keep B leaf untouched
    return asdict(box)["b"].run()
