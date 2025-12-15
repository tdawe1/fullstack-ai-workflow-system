"""Add MFA fields to users table.

Revision ID: 0006
Revises: 0005_add_task_archived_field
Create Date: 2025-12-15

"""
from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

# revision identifiers, used by Alembic.
revision = '0006_add_mfa_fields'
down_revision = '0005_add_task_archived_field'
branch_labels = None
depends_on = None


def upgrade() -> None:
    """Add MFA-related columns to users table."""
    op.add_column('users', sa.Column('mfa_enabled', sa.Boolean(), nullable=False, server_default='false'))
    op.add_column('users', sa.Column('mfa_secret', sa.String(), nullable=True))
    op.add_column('users', sa.Column('backup_codes', postgresql.JSONB(astext_type=sa.Text()), nullable=True))


def downgrade() -> None:
    """Remove MFA-related columns from users table."""
    op.drop_column('users', 'backup_codes')
    op.drop_column('users', 'mfa_secret')
    op.drop_column('users', 'mfa_enabled')
