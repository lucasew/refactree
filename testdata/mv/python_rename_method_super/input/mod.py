class Base:
    def helper(self):
        return 1

    def stay(self):
        return 2


class Child(Base):
    def run(self):
        return super().helper() + self.helper() + self.stay()
