from dataclasses import dataclass, asdict
import dataclasses


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A
    b: B


def use_list(box: Box) -> int:
    return list(asdict(box).values())[0].run()


def use_tuple(box: Box) -> int:
    return tuple(asdict(box).values())[0].run()


def use_dc_list(box: Box) -> int:
    return list(dataclasses.asdict(box).values())[0].run()


def use_vars_list(box: Box) -> int:
    return list(vars(box).values())[0].run()


def use_dunder_list(box: Box) -> int:
    return list(box.__dict__.values())[0].run()


def use_assigned(box: Box) -> int:
    d = asdict(box)
    return list(d.values())[0].run()


def use_assigned_vars(box: Box) -> int:
    d = vars(box)
    return list(d.values())[0].run()


def use_walrus(box: Box) -> int:
    if (d := asdict(box)):
        return list(d.values())[0].run()
    return 0


def use_field_var(box: Box) -> int:
    xa = list(asdict(box).values())[0]
    return xa.run()


def use_b_index(box: Box) -> int:
    # field 1 is B — foreign same-leaf must stay put
    return list(asdict(box).values())[1].run()


def use_index_still(box: Box) -> int:
    # keep B leaf untouched via key path
    return asdict(box)["b"].run()
