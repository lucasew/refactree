class Base:
    def helper(self):
        return 1


class Child(Base):
    def helper(self):
        return super().helper() + 1
